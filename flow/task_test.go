// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"errors"
	"testing"

	"golang.org/x/net/context"
)

func TestRun(t *testing.T) {
	Run(func(c Completion) {
		t.Fatal("expected not to be run")
	})
}

func TestActivate(t *testing.T) {
	run := make(chan struct{})

	ctx := context.Background()
	c := RunWithContext(func(c ContextCompletion) {
		if c.(*activateCompleter).Context != ctx {
			t.Errorf("Expected %#v, got: %#v", ctx, c.(context.Context))
		}
		close(run)
	})

	c.Activate(ctx)
	c.Wait()

	select {
	case <-run:
	default:
		t.Fatal("expected the worker to be run")
	}
}

func TestActivateNil(t *testing.T) {
	run := make(chan struct{})

	c := RunWithContext(func(c ContextCompletion) {
		if c.(*activateCompleter).Context != nil {
			t.Errorf("Expected %#v, got: %#v", nil, c)
		}
		close(run)
	})

	c.Activate(nil)
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

	c.Activate(nil)
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}
}

func TestCompleteWithError(t *testing.T) {
	c := Run(func(c Completion) {
		c.Complete(errors.New("test error"))
	})

	c.Activate(nil)
	_, err := c.Wait()
	if err == nil {
		t.Fatal("expected an error")
	}
}
