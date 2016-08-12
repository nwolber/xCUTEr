// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"

	"github.com/nwolber/xCUTEr/flow"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

type sshClient struct {
	c *ssh.Client
}

func newSSHClient(ctx context.Context, addr, user string) (*sshClient, error) {
	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

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
	l.Println("connecting to", addr)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		client.Close()
	}()
	l.Println("connected to", addr)
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
