// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/nwolber/xCUTEr/logger"
	errs "github.com/pkg/errors"
)

type acceptMsg struct {
	net.Conn
	error
}

func accept(ctx context.Context, listener net.Listener) <-chan acceptMsg {
	l, ok := ctx.Value(LoggerKey).(logger.Logger)
	if !ok || l == nil {
		l = logger.New(log.New(os.Stderr, "", log.LstdFlags), false)
	}

	c := make(chan acceptMsg)
	go func(l logger.Logger, c chan<- acceptMsg, listener net.Listener) {
		for {
			conn, err := listener.Accept()

			err = errs.Wrap(err, "failed to accept from listener")
			c <- acceptMsg{conn, err}

			if err != nil {
				l.Println("accept: listener errored", err)
				close(c)
				return
			}
		}
	}(l, c, listener)

	go func(l logger.Logger, ctx context.Context, listener net.Listener) {
		<-ctx.Done()
		l.Println("closing listener")
		listener.Close()
	}(l, ctx, listener)

	return c
}
