// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

func doSCP(ctx context.Context, privateKey []byte, addr string) error {
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

				go handleSSHConnection(ctx, conn, config)

			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func handleSSHConnection(ctx context.Context, nConn net.Conn, config *ssh.ServerConfig) {
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

			l.Println("New channel:", newChannel.ChannelType())

			if newChannel.ChannelType() != "session" {
				newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
				continue
			}
			channel, requests, err := newChannel.Accept()
			if err != nil {
				l.Println("could not accept channel")
				continue
			}

			go func(ctx context.Context, in <-chan *ssh.Request) {
				for {
					select {
					case req, ok := <-in:
						if !ok {
							return
						}

						l.Printf("%q requested: %q", req.Type, req.Payload)
						switch req.Type {
						case "exec":
							go handleExecRequest(ctx, channel, req)
						default:
							req.Reply(false, nil)
						}

					case <-ctx.Done():
						return
					}
				}
			}(ctx, requests)

		case <-ctx.Done():
			return
		}
	}
	l.Println("handleSSHConnection: connection closed")
}

func handleExecRequest(ctx context.Context, channel ssh.Channel, req *ssh.Request) {
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

	cmd := exec.Command(exe, parts[1:]...)
	cmd.Stdin = channel
	cmd.Stdout = channel

	processTerminated := make(chan int)

	go func(cmd *exec.Cmd, c chan<- int) {
		if err := cmd.Run(); err != nil {
			l.Println("error running", exe, err)

			// TODO get exit code from err.(*exec.ExitStatus)
			c <- 1
		} else {
			l.Println(exe, "completed successfully")
			c <- 0
		}
	}(cmd, processTerminated)

	var exitCode int
	select {
	case exitCode = <-processTerminated:
	case <-ctx.Done():
		l.Println("context completed, killing process")
		cmd.Process.Kill()
		exitCode = 255
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.BigEndian, int32(exitCode)); err != nil {
		l.Println("unable to convert int32 to byte")
		return
	}
	channel.SendRequest("exit-status", false, buf.Bytes())
}
