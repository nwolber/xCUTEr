// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"errors"
	"testing"
)

func TestParallel(t *testing.T) {
	done := make(chan struct{})

	i := 0
	go func(pI *int) {
		for ; i < 3; i++ {
			<-done
		}
	}(&i)

	c := Parallel(
		Run(func(c Completion) { done <- struct{}{} }),
		Run(func(c Completion) { done <- struct{}{} }),
		Run(func(c Completion) { done <- struct{}{} }),
	)

	c.Activate()
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatalf("expected 3 methods to run, got %d", i)
	}
}

func TestMultiActivation(t *testing.T) {
	done := make(chan struct{})

	i := 0
	go func(pI *int) {
		for ; i < 3; i++ {
			<-done
		}
	}(&i)

	c := Parallel(
		Run(func(c Completion) { done <- struct{}{} }),
		Run(func(c Completion) { done <- struct{}{} }),
		Run(func(c Completion) { done <- struct{}{} }),
	)

	c.Activate()
	c.Activate()
	c.Activate()
	c.Activate()
	_, err := c.Wait()
	if err != nil {
		t.Fatal("expected no error, got:", err)
	}

	if i != 3 {
		t.Fatalf("expected 3 methods to run, got %d", i)
	}
}

func TestParallelWithError(t *testing.T) {
	c := Parallel(
		Run(func(c Completion) {}),
		Run(func(c Completion) { c.Complete(errors.New("test error")) }),
		Run(func(c Completion) {}),
	)

	c.Activate()
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
