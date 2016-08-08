// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"errors"
	"testing"
)

func TestRun(t *testing.T) {
	Run(func(c Completion) {
		t.Fatal("expected not to be run")
	})
}

func TestActivate(t *testing.T) {
	run := make(chan struct{})

	c := Run(func(c Completion) {
		close(run)
	})

	c.Activate()
	c.Wait()

	select {
	case <-run:
	default:
		t.Fatal("expected the worker to be run")
	}
}

func TestComplete(t *testing.T) {
	c := Run(func(c Completion) {
		c.Complete(nil)
	})

	c.Activate()
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}
}

func TestCompleteWithError(t *testing.T) {
	c := Run(func(c Completion) {
		c.Complete(errors.New("test error"))
	})

	c.Activate()
	_, err := c.Wait()
	if err == nil {
		t.Fatal("expected an error")
	}
}
