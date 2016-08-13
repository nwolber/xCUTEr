// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"log"
	"net"

	"golang.org/x/net/context"
)

type acceptMsg struct {
	net.Conn
	error
}

func accept(ctx context.Context, l net.Listener) <-chan acceptMsg {
	c := make(chan acceptMsg)
	go func(c chan<- acceptMsg, l net.Listener) {
		for {
			conn, err := l.Accept()

			c <- acceptMsg{conn, err}

			if err != nil {
				log.Println("accept: listener errored", err)
				close(c)
				return
			}
		}
	}(c, l)

	go func(ctx context.Context, l net.Listener) {
		<-ctx.Done()
		logger, ok := ctx.Value(loggerKey).(*log.Logger)
		if !ok {
			err := fmt.Errorf("no %s available", loggerKey)
			log.Println(err)
			return
		}
		logger.Println("closing listener")
		l.Close()
	}(ctx, l)

	return c
}
