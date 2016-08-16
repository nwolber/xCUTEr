// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
	"context"
)

// forward instructs the connect SSH server to forward all connections attempts
// on remoteAddr to the local client. The client will then establish a connection
// to localAddr and forward any payload exchanged.
//
// Allocated resources will be released, when the context completes.
func forward(ctx context.Context, client *ssh.Client, remoteAddr string, localAddr string) {
	l, ok := ctx.Value(loggerKey).(*log.Logger)
	if !ok || l == nil {
		l = log.New(os.Stderr, "", log.LstdFlags)
	}

	listener, err := client.Listen("tcp", remoteAddr)
	if err != nil {
		err = fmt.Errorf("Unable to listen to %s on remote host %s: %s", remoteAddr, client.RemoteAddr(), err)
		l.Println(err)
		return
	}

	go func() {
		acceptChan := accept(ctx, listener)

		for {
			select {
			case remoteConn, ok := <-acceptChan:
				if !ok {
					l.Println("accept channel closed")
					return
				}

				if remoteConn.error != nil {
					l.Println("error accepting connection", err)
					return
				}

				go func(conn net.Conn) {
					defer conn.Close()
					l.Println("accepted connection from remote tunnel")

					localConn, err := net.Dial("tcp", localAddr)
					if err != nil {
						l.Println("unable to connect to local endpoint", localAddr, err)
						return
					}
					l.Println("connected to local endpoint")

					go copyConn(localConn, conn)
					copyConn(conn, localConn)
					l.Println("tunnel connection closed")
				}(remoteConn)

			case <-ctx.Done():
				l.Println("closing tunnel")
				return
			}
		}
	}()
}

func copyConn(writer io.Writer, reader io.Reader) {
	_, err := io.Copy(writer, reader)
	if err != nil {
		log.Println("io.Copy error:", err)
	}
}
