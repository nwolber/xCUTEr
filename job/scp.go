// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/nwolber/xCUTEr/scp"

	"golang.org/x/crypto/ssh"
)

func doSCP(ctx context.Context, privateKey []byte, addr string, verbose bool) error {
	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

	config := &ssh.ServerConfig{
		KeyboardInteractiveCallback: func(c ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			l.Println("client authenticated by keyboard interactive challenge")
			return nil, nil
		},
		PasswordCallback: func(s ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			l.Println("client authenticated by password challenge")
			return nil, nil
		},
		PublicKeyCallback: func(s ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			l.Println("client authenticated by public key challenge")
			return nil, nil
		},
	}

	private, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		err = fmt.Errorf("failed to parse private key %s", err)
		l.Println(err)
		return err
	}

	config.AddHostKey(private)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		err = fmt.Errorf("failed to listen for connection %s", err)
		l.Println(err)
		return err
	}

	go func() {
		acceptChan := accept(ctx, listener)

		for {
			select {
			case conn, ok := <-acceptChan:
				if !ok {
					l.Println("accept channel closed")
					return
				}

				if conn.error != nil {
					l.Println("error accepting connection", err)
					return
				}
				l.Println("accepted new connection")

				go handleSSHConnection(ctx, conn, config, verbose)

			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func handleSSHConnection(ctx context.Context, nConn net.Conn, config *ssh.ServerConfig, verbose bool) {
	defer nConn.Close()

	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

	_, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		l.Println("failed to handshake:", err)
		return
	}
	l.Println("handshake successful")

	go ssh.DiscardRequests(reqs)

	for {
		select {
		case newChannel, ok := <-chans:
			if !ok {
				return
			}

			if verbose {
				l.Println("New channel:", newChannel.ChannelType())
			}

			if newChannel.ChannelType() != "session" {
				newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				l.Println("could not accept channel")
				continue
			}

			go serveRequests(ctx, channel, requests, verbose)
		case <-ctx.Done():
			return
		}
	}
	l.Println("handleSSHConnection: connection closed")
}

func serveRequests(ctx context.Context, channel ssh.Channel, in <-chan *ssh.Request, verbose bool) {
	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

	for {
		select {
		case req, ok := <-in:
			if !ok {
				return
			}

			if verbose {
				l.Printf("%q requested: %q", req.Type, req.Payload)
			}
			switch req.Type {
			case "exec":
				go handleExecRequest(ctx, channel, req, verbose)
			default:
				req.Reply(false, nil)
			}

		case <-ctx.Done():
			return
		}
	}
}

func handleExecRequest(ctx context.Context, channel ssh.Channel, req *ssh.Request, verbose bool) {
	defer channel.Close()

	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

	parts := strings.Split(string(req.Payload), " ")
	exe := parts[0][4:]

	if exe != "scp" {
		l.Println("remote requested", exe, "denying")
		req.Reply(false, nil)
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
	}

	exitCode := 0
	err := scp.New(string(req.Payload[4:]), channel, channel)
	if err != nil {
		l.Println("error during scp transfer", err)
		exitCode = 1
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, int32(exitCode)); err != nil {
		l.Println("unable to convert int32 to byte")
		return
	}
	channel.SendRequest("exit-status", false, buf.Bytes())
}

func oldSCP(ctx context.Context, channel ssh.Channel, req *ssh.Request, verbose bool) {
	defer channel.Close()

	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

	parts := strings.Split(string(req.Payload), " ")
	exe := parts[0][4:]

	if exe != "scp" {
		l.Println("remote requested", exe, "denying")
		req.Reply(false, nil)
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
	}

	cmd := exec.CommandContext(ctx, exe, parts[1:]...)
	cmd.Stdin = channel
	cmd.Stdout = channel

	if verbose {
		cmd.Stderr = os.Stderr
	}

	exitCode := 0
	if err := cmd.Run(); err != nil {
		l.Println("error running", exe, err)
		exitCode = 1

		// TODO get exit code from err.(*exec.ExitStatus)
	} else {
		l.Println(exe, "completed successfully")
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, int32(exitCode)); err != nil {
		l.Println("unable to convert int32 to byte")
		return
	}
	channel.SendRequest("exit-status", false, buf.Bytes())
}

type bla struct {
	dir string
}

func (x *bla) Write(b []byte) (int, error) {
	log.Printf("!!!!!!!!!!!!!!!!!!!! %s: %q", x.dir, string(b))
	return len(b), nil
}
