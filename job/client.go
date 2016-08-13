// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/nwolber/xCUTEr/flow"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

type storeElement struct {
	ref    int
	client *sshClient
}

type sshClientStore struct {
	clients map[string]*storeElement
	m       sync.Mutex
}

var (
	store = &sshClientStore{
		clients: make(map[string]*storeElement),
	}
)

func newSSHClient(ctx context.Context, addr, user string) (*sshClient, error) {
	key := fmt.Sprintf("%s@%s", user, addr)

	store.m.Lock()
	defer store.m.Unlock()

	elem, ok := store.clients[key]

	if !ok {
		client, err := createClient(addr, user)
		if err != nil {
			return nil, err
		}

		store.clients[key] = &storeElement{
			ref:    1,
			client: client,
		}

		go func(ctx context.Context, client *sshClient) {
			<-ctx.Done()
			store.m.Lock()
			defer store.m.Unlock()

			elem := store.clients[key]
			elem.ref--

			if elem.ref == 0 {
				elem.client.c.Close()
				log.Println("connection to", addr, "closed")
				delete(store.clients, key)
			}
		}(ctx, client)

		return client, nil
	}

	elem.ref++
	return elem.client, nil
}

type sshClient struct {
	c *ssh.Client
}

func createClient(addr, user string) (*sshClient, error) {
	password, err := ioutil.ReadFile("password")
	if err != nil {
		log.Panicln(err)
	}

	s, _, err := readPrivateKeyFile("/Users/niklas/.ssh/niklas", password)
	if err != nil {
		log.Fatalln("Unable to read private key", err)
	}

	signer, err := ssh.NewSignerFromSigner(s)
	if err != nil {
		log.Fatalln("Unable to turn signer into signer", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
	}
	log.Println("connecting to", addr)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	// go func() {
	// 	<-ctx.Done()
	// 	client.Close()
	// }()
	log.Println("connected to", addr)
	return &sshClient{
		c: client,
	}, nil
}

func (s *sshClient) executeCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

	select {
	case <-ctx.Done():
		l.Printf("won't execute %q because context is done", command)
		return nil
	default:
	}

	session, err := s.c.NewSession()
	if err != nil {
		l.Println("failed to create session:", err)
	}
	defer session.Close()

	if stdout != nil {
		session.Stdout = stdout
	}

	if stderr != nil {
		session.Stderr = stderr
	}

	l.Printf("executing %q", command)
	if err := session.Start(command); err != nil {
		l.Printf("failed to start: %q, %s", command, err)
	}

	done := flow.New()
	go func() {
		done.Complete(session.Wait())
	}()

	select {
	case <-ctx.Done():
		l.Println("closing session, context done")
		return nil
	case err, _ := <-done.Chan():
		if err != nil {
			l.Printf("executing %q failed: %s", command, err)
			return err
		}
	}

	l.Printf("%q executed successfully", command)
	return nil
}

func (s *sshClient) forward(ctx context.Context, remoteAddr, localAddr string) {
	forward(ctx, s.c, remoteAddr, localAddr)
}
