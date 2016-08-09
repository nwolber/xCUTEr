// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

import "golang.org/x/net/context"

// Activater activates a task.
type Activater interface {
	Activate(context.Context) Waiter
}

// A Task can be activated. It signals completion and can be waited for.
type Task interface {
	Waiter
	Activater
}

// A DeferTask is executed although a previous task failed.
type DeferTask interface {
	Defer()
}

// A GroupTask is a Task that consists of multiple child tasks.
type GroupTask interface {
	Task
	Add(child Task)
}

type activateCompleter struct {
	complete Completion
	activate Completion

	context.Context
}

func (c *activateCompleter) Complete(err error) {
	c.complete.Complete(err)
}

func (c *activateCompleter) Wait() (bool, error) {
	return c.complete.Wait()
}

func (c *activateCompleter) Activate(ctx context.Context) Waiter {
	c.Context = ctx
	c.activate.Complete(nil)
	return c
}

// Run returns a task that, when activated executes f.
// The task complete, when f returns, encounters an error or
// completes prematurely.
func Run(f func(c Completion)) Task {
	c := &activateCompleter{
		complete: New(),
		activate: New(),
	}

	go func(c *activateCompleter, f func(c Completion)) {
		c.activate.Wait()
		f(c)
		c.complete.Complete(nil)
	}(c, f)
	return c
}

func RunWithContext(f func(c ContextCompletion)) Task {
	c := &activateCompleter{
		complete: New(),
		activate: New(),
	}

	go func(c *activateCompleter, f func(c ContextCompletion)) {
		c.activate.Wait()
		f(c)
		c.complete.Complete(nil)
	}(c, f)
	return c
}

type deferCompleter struct {
	activateCompleter
}

func (c *deferCompleter) Defer() {}

func Defer(f func(Completion)) DeferTask {
	c := &deferCompleter{
		activateCompleter{
			complete: New(),
			activate: New(),
		},
	}

	go func(c *deferCompleter, f func(c Completion)) {
		c.activate.Wait()
		f(c)
		c.complete.Complete(nil)
	}(c, f)
	return c
}
