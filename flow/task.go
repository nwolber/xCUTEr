// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flow

// Activater activates a task.
type Activater interface {
	Activate() Waiter
}

// A Task can be activated. It signals completion and can be waited for.
type Task interface {
	Waiter
	Activater
}

type activateCompleter struct {
	complete Completion
	activate Completion
}

func (c *activateCompleter) Complete(err error) {
	c.complete.Complete(err)
}

func (c *activateCompleter) Wait() (bool, error) {
	return c.complete.Wait()
}

func (c *activateCompleter) Activate() Waiter {
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
