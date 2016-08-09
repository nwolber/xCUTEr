// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"errors"
	"sync/atomic"
	"testing"

	"golang.org/x/net/context"
)

func TestSequential(t *testing.T) {
	var i int32
	pI := &i

	c := Sequential(
		Run(func(c Completion) {
			if atomic.AddInt32(pI, 1) != 1 {
				t.Fatalf("expected to be the first in line, actually: %d", 1)
			}
		}),
		Run(func(c Completion) {
			if atomic.AddInt32(pI, 1) != 2 {
				t.Fatalf("expected to be the second in line, actually: %d", 1)
			}
		}),
	)

	c.Add(Run(func(c Completion) {
		if atomic.AddInt32(pI, 1) != 3 {
			t.Fatalf("expected to be the third in line, actually: %d", 1)
		}
	}))

	c.Activate(nil)
	c.Activate(nil)
	c.Activate(nil)
	c.Activate(nil)
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatal("expected 3 tasks to run, got:", i)
	}
}

func TestSequentialWithError(t *testing.T) {
	var i int32
	pI := &i

	c := Sequential(
		Run(func(c Completion) {
			if atomic.AddInt32(pI, 1) != 1 {
				t.Fatalf("expected to be the first in line, actually: %d", 1)
			}
		}),
		Run(func(c Completion) {
			c.Complete(errors.New("test error"))
		}),
	)

	c.Add(Run(func(c Completion) {
		t.Fatal("expected to not being called")
	}))

	c.Activate(nil)
	_, err := c.Wait()
	if err == nil {
		t.Fatal("expected an error")
	}

	if i != 1 {
		t.Fatal("expected 1 task to run, got:", i)
	}
}

func TestSequentialContext(t *testing.T) {
	var i int32
	pI := &i

	ctx := context.Background()
	c := Sequential(
		RunWithContext(func(c ContextCompletion) {
			if c.(*activateCompleter).Context != ctx {
				t.Errorf("Expected %#v, got %#v", ctx, c)
			}

			ctx := context.WithValue(ctx, "test", 42)
			ctx.Value("test")
			if atomic.AddInt32(pI, 1) != 1 {
				t.Fatalf("expected to be the first in line, actually: %d", 1)
			}
		}),
		RunWithContext(func(c ContextCompletion) {
			if c.(*activateCompleter).Context != ctx {
				t.Errorf("Expected %#v, got %#v", ctx, c)
			}

			if atomic.AddInt32(pI, 1) != 2 {
				t.Fatalf("expected to be the second in line, actually: %d", 1)
			}
		}),
	)

	c.Add(RunWithContext(func(c ContextCompletion) {
		if c.(*activateCompleter).Context != ctx {
			t.Errorf("Expected %#v, got %#v", ctx, c)
		}
		if atomic.AddInt32(pI, 1) != 3 {
			t.Fatalf("expected to be the third in line, actually: %d", 1)
		}
	}))

	c.Activate(ctx)
	c.Activate(ctx)
	c.Activate(ctx)
	c.Activate(ctx)
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatal("expected 3 tasks to run, got:", i)
	}
}

func TestSequentialStore(t *testing.T) {
	ctx := context.Background()
	c := Sequential(
		RunWithContext(func(c ContextCompletion) {
			c = context.WithValue(c, "test", 123)
		}),
		RunWithContext(func(c ContextCompletion) {
			got := c.Value("test")
			if got != 123 {
				t.Errorf("want: %#v, got: %#v", want, got)
			}
		}),
	)

	c.Add(RunWithContext(func(c ContextCompletion) {

	}))

	c.Activate(ctx)

	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatal("expected 3 tasks to run, got:", i)
	}
}
