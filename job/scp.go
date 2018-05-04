// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"bytes"
	"context"
	"encoding/binary"
	"log"
	"net"
	"os"
	"strings"

	"github.com/nwolber/xCUTEr/logger"
	"github.com/nwolber/xCUTEr/scp"
	errs "github.com/pkg/errors"

	"golang.org/x/crypto/ssh"
)

func doSCP(ctx context.Context, privateKey []byte, addr string, verbose bool) error {
	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
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
		err = errs.Wrap(err, "failed to parse private key")
		l.Println(err)
		return err
	}

	config.AddHostKey(private)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		err = errs.Wrapf(err, "failed to listen for connection on %s", addr)
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
	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
	}

	defer l.Println("handleSSHConnection: connection closed")

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
}

func serveRequests(ctx context.Context, channel ssh.Channel, in <-chan *ssh.Request, verbose bool) {
	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
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

	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
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
	err := scp.NewLogger(string(req.Payload[4:]), channel, channel, verbose, l)
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
