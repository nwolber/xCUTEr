// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import (
	"log"
	"sync"
)

// SequentialTask implements sequential execution of children
type SequentialTask struct {
	parent   Completion
	children []Task

	active bool
	m      sync.Mutex
}

// Sequential returns a new task that executes its children in the given order.
func Sequential(children ...Task) *SequentialTask {
	c := &SequentialTask{
		parent:   New(),
		children: children,
	}

	return c
}

// Wait waits until all children have completed
func (c *SequentialTask) Wait() (bool, error) {
	return c.parent.Wait()
}

// Activate starts execution of the children.
func (c *SequentialTask) Activate() Waiter {
	c.m.Lock()
	defer c.m.Unlock()

	if c.active {
		return c
	}

	c.active = true

	go func() {
		for _, child := range c.children {
			child.Activate()
			if _, err := child.Wait(); err != nil {
				c.parent.Complete(err)
				return
			}
		}
		c.parent.Complete(nil)
	}()
	return c
}

// Add adds a new child, which will be executed last.
// Add panics if the task is already activated.
func (c *SequentialTask) Add(child Task) {
	c.m.Lock()
	defer c.m.Unlock()

	if c.active {
		log.Panicln("Sequential is active")
	}

	if child == nil {
		log.Panicln("child is nil")
	}

	c.children = append(c.children, child)
}
