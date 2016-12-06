// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type storeElement struct {
	ref      int
	client   *sshClient
	lastUsed time.Time
}

type sshClientStore struct {
	clients map[string]*storeElement
	m       sync.Mutex
}

var (
	errUnknownUser = errors.New("keyboard interactive: unknown user")

	store *sshClientStore

	// KeepAliveInterval between two keep-alive messages.
	// Changes to the time interval only apply to newly
	// created connections.
	KeepAliveInterval = 30 * time.Second
)

// InitializeSSHClientStore initialies the global SSH connection store and
// sets the time-to-live for unused connections.
func InitializeSSHClientStore(ttl time.Duration) {
	store = &sshClientStore{
		clients: make(map[string]*storeElement),
	}

	// This go routine runs for the lifetime of the program.
	go func() {
		for {
			watchTime := time.Duration(float64(ttl.Nanoseconds()) * 0.1)
			<-time.After(watchTime)

			func() {
				store.m.Lock()
				defer store.m.Unlock()

				for key, elem := range store.clients {
					if diff := time.Now().Sub(elem.lastUsed); elem.ref <= 0 && diff > ttl {
						log.Println("connection to", key, "unused for", diff, "closing")
						elem.client.c.Close()
						delete(store.clients, key)
						close(elem.client.trashed)
					}
				}
			}()
		}
	}()
}

func waitConn(client *ssh.Client) chan error {
	c := make(chan error)
	go func(client *ssh.Client, c chan error) {
		c <- client.Wait()
		close(c)
	}(client, c)

	return c
}

func newSSHClient(ctx context.Context, addr, user, keyFile, password string, keyboardInteractive map[string]string) (*sshClient, error) {
	key := fmt.Sprintf("%s@%s", user, addr)

	elem, ok := func() (*storeElement, bool) {
		store.m.Lock()
		defer store.m.Unlock()

		elem, ok := store.clients[key]
		return elem, ok
	}()

	if !ok {
		client, err := createClient(addr, user, keyFile, password, keyboardInteractive)
		if err != nil {
			return nil, err
		}

		go func(client *sshClient) {
			connClosed := waitConn(client.c)
			keepAliveTimer := time.NewTicker(KeepAliveInterval)
			defer keepAliveTimer.Stop()

		loop:
			for {
				select {
				case <-keepAliveTimer.C:
					log.Println("sending keep-alive request to", key)
					_, _, err := client.c.SendRequest("xCUTEr keep-alive", true, nil)
					if err != nil {
						log.Println("keep-alive to", key, "failed:", err)
						continue
					}
					log.Println("keep-alive to", key, "successful")
				case err := <-connClosed:
					if err != nil {
						log.Println("connection to", key, "closed, removing from store:", err)
					} else {
						log.Println("conntection to", key, "closed without error, removing from store")
					}
					break loop
				}
			}

			store.m.Lock()
			defer store.m.Unlock()
			if _, ok := store.clients[key]; ok {
				delete(store.clients, key)
			}
			close(client.trashed)
		}(client)

		store.m.Lock()
		defer store.m.Unlock()

		elem = &storeElement{
			client: client,
		}
		store.clients[key] = elem
	} else {
		store.m.Lock()
		defer store.m.Unlock()

		log.Println("reusing existing connection to", key)
	}

	elem.ref++
	elem.lastUsed = time.Now()
	log.Println("incrementing ref counter for", key, "new value:", elem.ref)

	go func(ctx context.Context, client *sshClient) {
		<-ctx.Done()
		store.m.Lock()
		defer store.m.Unlock()

		elem, ok := store.clients[key]
		if ok {
			elem.ref--
			elem.lastUsed = time.Now()
			log.Println("context done, decrementing ref counter for", key, "new value:", elem.ref)
		} else {
			log.Println("context done, connection to", key, "doesn't exist anymore")
		}
	}(ctx, elem.client)

	return elem.client, nil
}

type sshClient struct {
	c       *ssh.Client
	trashed chan struct{}
}

var createClient = func(addr, user, keyFile, password string, keyboardInteractive map[string]string) (*sshClient, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{},
	}

	if keyFile != "" {
		s, _, err := readPrivateKeyFile(keyFile, nil)
		if err != nil {
			err = fmt.Errorf("Unable to read private key %s", err)
			log.Println(err)
			return nil, err
		}

		signer, err := ssh.NewSignerFromSigner(s)
		if err != nil {
			err = fmt.Errorf("Unable to turn signer into signer %s", err)
			log.Println(err)
			return nil, err
		}
		config.Auth = append(config.Auth, ssh.PublicKeys(signer))
	}

	if password != "" {
		config.Auth = append(config.Auth, ssh.Password(password))
	}

	if keyboardInteractive != nil && len(keyboardInteractive) > 0 {
		config.Auth = append(config.Auth, ssh.KeyboardInteractive(keyboardInteractiveChallenge(user, keyboardInteractive)))
	}

	log.Println("no existing connection, connecting to", addr)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	log.Println("connected to", addr)
	return &sshClient{
		c:       client,
		trashed: make(chan struct{}),
	}, nil
}

func keyboardInteractiveChallenge(user string, keyboardInteractive map[string]string) ssh.KeyboardInteractiveChallenge {
	return func(challengeUser, instruction string, questions []string, echos []bool) ([]string, error) {
		if len(questions) == 0 {
			return nil, nil
		}

		var answers []string
		for _, question := range questions {
			if answer, ok := keyboardInteractive[question]; ok {
				answers = append(answers, answer)
			}
		}

		return answers, nil
	}
}

func (s *sshClient) executeCommand(ctx context.Context, command string, stdout, stderr io.Writer) error {
	l, ok := ctx.Value(loggerKey).(Logger)
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
		return err
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

	done := make(chan error)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-ctx.Done():
		l.Println("closing session, context done")
		return nil
	case err, _ := <-done:
		if err != nil {
			l.Printf("executing %q failed: %s", command, err)
			return err
		}
	}

	l.Printf("%q executed successfully", command)
	return nil
}

func (s *sshClient) forwardRemote(ctx context.Context, remoteAddr, localAddr string) {
	forwardRemote(ctx, s.c, remoteAddr, localAddr)
}

func (s *sshClient) forwardTunnel(ctx context.Context, remoteAddr, localAddr string) {
	forwardLocal(ctx, s.c, remoteAddr, localAddr)
}
