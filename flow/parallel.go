// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"log"
	"sync"
)

// ParallelTask executes its children in parallel.
type ParallelTask struct {
	parent   Completion
	activate Completion
	done     chan struct{}

	n      int
	active bool
	lock   sync.Mutex
}

// Parallel returns a new task that executes its children in parallel.
func Parallel(children ...Task) *ParallelTask {
	c := &ParallelTask{
		parent:   New(),
		activate: New(),
		done:     make(chan struct{}),
	}

	for _, child := range children {
		c.Add(child)
	}

	return c
}

// Activate starts parallel execution of all children.
func (c *ParallelTask) Activate() Waiter {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.active {
		return c
	}

	c.activate.Complete(nil)

	go c.loop()
	c.active = true
	return c
}

// Wait until the children completed.
func (c *ParallelTask) Wait() (bool, error) {
	return c.parent.Wait()
}

// Insert adds a new completer without increasing the number of child completers
// that have to complete, before the multi completer completes.
func (c *ParallelTask) Insert(child Task) {
	if child == nil {
		log.Panicln("child is nil")
	}

	go c.watchChild(child)
}

// Add adds a new completer and increases the number of child completers
// that have to complete, before the multi completer completes.
func (c *ParallelTask) Add(child Task) {
	if child == nil {
		log.Panicln("child is nil")
	}

	c.addN(1)
	go c.watchChild(child)
}

func (c *ParallelTask) addN(a int) int {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.n += a
	return c.n
}

func (c *ParallelTask) loop() {
	for i := 0; i < c.addN(0); i++ {
		<-c.done
	}
	c.parent.Complete(nil)
}

func (c *ParallelTask) watchChild(child Task) {
	var (
		ok  bool
		err error
	)

	c.activate.Wait()
	child.Activate()

	for ok = true; ok && err == nil; {
		ok, err = child.Wait()
	}

	if err != nil {
		c.parent.Complete(err)
	}
	c.done <- struct{}{}
}
