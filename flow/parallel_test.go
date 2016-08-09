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

func TestParallel(t *testing.T) {
	var i int32
	pI := &i

	c := Parallel(
		Run(func(c Completion) { atomic.AddInt32(pI, 1) }),
		Run(func(c Completion) { atomic.AddInt32(pI, 1) }),
		Run(func(c Completion) { atomic.AddInt32(pI, 1) }),
	)

	c.Activate(nil)
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatalf("expected 3 methods to run, got %d", i)
	}
}

func TestMultiActivation(t *testing.T) {
	var i int32
	pI := &i

	c := Parallel(
		Run(func(c Completion) { atomic.AddInt32(pI, 1) }),
		Run(func(c Completion) { atomic.AddInt32(pI, 1) }),
		Run(func(c Completion) { atomic.AddInt32(pI, 1) }),
	)

	c.Activate(nil)
	c.Activate(nil)
	c.Activate(nil)
	c.Activate(nil)
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatalf("expected 3 methods to run, got %d", i)
	}
}

func TestParallelWithError(t *testing.T) {
	ctx := context.Background()
	c := Parallel(
		Run(func(c Completion) {}),
		Run(func(c Completion) { c.Complete(errors.New("test error")) }),
		Run(func(c Completion) {}),
	)

	c.Activate(ctx)
	_, err := c.Wait()
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestParallelAdd(t *testing.T) {
	c := Parallel()
	c.Add(Run(func(c Completion) {}))

	if c.n != 1 {
		t.Fatal("expected n to be 1, got", c.n)
	}
}

func TestParallelInsert(t *testing.T) {
	c := Parallel()
	c.Insert(Run(func(c Completion) {}))

	if c.n != 0 {
		t.Fatal("expected n to be 0, got", c.n)
	}
}

func TestParallelContext(t *testing.T) {
	var i int32
	pI := &i

	ctx := context.Background()
	c := Parallel(
		RunWithContext(func(c ContextCompletion) {
			if c.(*activateCompleter).Context != ctx {
				t.Errorf("Expected %#v, got %#v", ctx, c)
			}
			atomic.AddInt32(pI, 1)
		}),
		RunWithContext(func(c ContextCompletion) {
			if c.(*activateCompleter).Context != ctx {
				t.Errorf("Expected %#v, got %#v", ctx, c)
			}
			atomic.AddInt32(pI, 1)
		}),
		RunWithContext(func(c ContextCompletion) {
			if c.(*activateCompleter).Context != ctx {
				t.Errorf("Expected %#v, got %#v", ctx, c)
			}
			atomic.AddInt32(pI, 1)
		}),
	)

	c.Activate(ctx)
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatalf("expected 3 methods to run, got %d", i)
	}
}
