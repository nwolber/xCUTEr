// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"errors"
	"testing"
	"time"
)

func TestNoCompletion(t *testing.T) {
	c := New()
	done := make(chan struct{})

	go func() {
		select {
		case <-c.Chan():
			t.Fatal("Completer should not complete without Complete begin called")
		case <-done:
		}
	}()

	close(done)
}

func TestCompletion(t *testing.T) {
	c := New()
	c.Complete(nil)

	select {
	case err := <-c.Chan():
		if err != nil {
			t.Fatal("expected no error, got:", err)
		}
	default:
		t.Fatal("expected Completer to be completed")
	}
}

func TestCompletionWithError(t *testing.T) {
	c := New()
	go c.Complete(errors.New("test error"))

	go func() {
		select {
		case err := <-c.Chan():
			if err == nil {
				t.Fatal("expected an error")
			}
		default:
			t.Fatal("expected Completer to be completed")
		}
	}()
}

func TestMultiCompletion(t *testing.T) {
	c := New()

	go c.Complete(errors.New("test error"))
	c.Complete(nil)

	go func() {
		select {
		case err := <-c.Chan():
			if err == nil {
				t.Fatal("expected an error")
			}
		default:
			t.Fatal("expected Completer to be completed")
		}
	}()
}

func TestWait(t *testing.T) {
	c := New()
	done := make(chan struct{})

	go c.Complete(errors.New("test error"))
	go func() {
		defer close(done)
		_, err := c.Wait()
		if err == nil {
			t.Fatal("expected an error")
		}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected Completer to be completed")
	}
}
