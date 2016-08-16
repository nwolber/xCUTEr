// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"log"

	"context"
)

// Completer signals completion.
type Completer interface {
	Complete(err error)
}

// Waiter waits for the object to complete.
type Waiter interface {
	Wait() (bool, error)
}

// Completion allows to signal and wait for completion of a task
type Completion interface {
	Completer
	Waiter
}

type ContextCompletion interface {
	Completion
	context.Context
}

// SimpleCompleter signals completion by closing the channel.
// Optionally an error can be transmitted before completing.
type SimpleCompleter chan error

// New returns a new Completer
func New() SimpleCompleter {
	return make(SimpleCompleter)
}

// Complete signals completion, optionally with an error.
func (c SimpleCompleter) Complete(err error) {
	select {
	case e, ok := <-c:
		if e != nil && ok {
			select {
			case c <- e:
			default:
				log.Println("unable to send error:", err)
			}
		}
		return
	default:
	}

	if err != nil {
		// BUG: possible race, if two go routines complete simultaniously
		// writing to a closed channel.
		c <- err
	}

	close(c)
}

// Wait until completion.
func (c SimpleCompleter) Wait() (bool, error) {
	err, ok := <-c
	return ok, err
}

// Chan is a convenience method to retrieve the underlying channel.
func (c SimpleCompleter) Chan() <-chan error {
	return c
}
