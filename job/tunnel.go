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

	"golang.org/x/crypto/ssh"
)

// forwardRemote instructs the connect SSH server to forward all connections attempts
// on remoteAddr to the local client. The client will then establish a connection
// to localAddr and forward any payload exchanged.
//
// Allocated resources will be released, when the context completes.
func forwardRemote(ctx context.Context, client *ssh.Client, remoteAddr string, localAddr string) {
	l, ok := ctx.Value(loggerKey).(Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
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
	l, ok := ctx.Value(loggerKey).(Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
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
	l, ok := ctx.Value(loggerKey).(Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
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
				l.Println("accepted tunnel connection")

				localConn, err := d("tcp", addr)
				if err != nil {
					l.Println("unable to connect to endpoint", addr, err)
					return
				}
				l.Println("connected to endpoint")

				go copyConn(localConn, conn)
				copyConn(conn, localConn)
				l.Println("tunnel connection closed")
			}(remoteConn)

		case <-ctx.Done():
			l.Println("closing tunnel")
			return
		}
	}
}

func copyConn(writer io.Writer, reader io.Reader) {
	_, err := io.Copy(writer, reader)
	if err != nil {
		log.Println("io.Copy error:", err)
	}
}
