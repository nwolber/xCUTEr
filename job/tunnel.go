// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/nwolber/xCUTEr/logger"

	"golang.org/x/crypto/ssh"
)

// forwardRemote instructs the connect SSH server to forward all connections attempts
// on remoteAddr to the local client. The client will then establish a connection
// to localAddr and forward any payload exchanged.
//
// Allocated resources will be released, when the context completes.
func forwardRemote(ctx context.Context, client *ssh.Client, remoteAddr string, localAddr string) {
	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
	}

	listener, err := client.Listen("tcp", remoteAddr)
	if err != nil {
		err = fmt.Errorf("unable to listen to %s on remote host %s: %s", remoteAddr, client.RemoteAddr(), err)
		l.Println(err)
		return
	}

	go runTunnel(ctx, listener, net.Dial, localAddr)
}

// forwardLocal forwards all connection attempts on localAddr to the remote host client
// connects to. The remote host will then establish a connection remoteAddr.
//
// Allocated resources will be released, when the context completes.
func forwardLocal(ctx context.Context, client *ssh.Client, remoteAddr string, localAddr string) {
	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
	}

	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		err = fmt.Errorf("unable to listen to %s: %s", localAddr, err)
		l.Println(err)
		return
	}

	go runTunnel(ctx, listener, client.Dial, remoteAddr)
}

type dial func(network, address string) (net.Conn, error)

func runTunnel(ctx context.Context, listener net.Listener, d dial, addr string) {
	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
	}

	acceptChan := accept(ctx, listener)

	for {
		select {
		case remoteConn, ok := <-acceptChan:
			if !ok {
				l.Println("accept channel closed")
				return
			}

			if remoteConn.error != nil {
				l.Println("error accepting tunnel connection", remoteConn.error)
				return
			}

			go func(conn net.Conn) {
				defer conn.Close()
				identity := fmt.Sprintf("%s->%s", conn.RemoteAddr(), conn.LocalAddr())

				l.Println("accepted tunnel connection", identity)

				localConn, err := d("tcp", addr)
				if err != nil {
					l.Println(identity, "unable to connect to endpoint", addr, err)
					return
				}
				l.Println(identity, "connected to endpoint", addr)

				go copyConn(identity, localConn, conn)
				copyConn(identity, conn, localConn)
				l.Println(identity, "tunnel connection closed to", addr)
			}(remoteConn)

		case <-ctx.Done():
			l.Println("context done, closing tunnel on", listener.Addr())
			return
		}
	}
}

func copyConn(identity string, writer io.Writer, reader io.Reader) {
	_, err := io.Copy(writer, reader)
	if err != nil {
		log.Println(identity, "io.Copy error:", err)
	}
}
