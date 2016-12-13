// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"context"
	"time"

	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
	"github.com/nwolber/xCUTEr/logger"
)

type EventType uint

const (
	EventStart EventType = iota
	EventLog
	EventEnd
	EventFailed
)

func (e EventType) String() string {
	switch e {
	case EventStart:
		return "Start"
	case EventLog:
		return "Log"
	case EventEnd:
		return "End"
	case EventFailed:
		return "Failed"
	}
	return "Unknown"
}

type Event struct {
	Type      EventType
	Timestamp time.Time
	Name      string
	Info      interface{}
}

func StoreEvents(store *[]Event, events <-chan Event, done chan struct{}) {
	for event := range events {
		*store = append(*store, event)
	}
	if done != nil {
		close(done)
	}
}

func instrument(name string, f flunc.Flunc, events chan<- Event) flunc.Flunc {
	if f == nil {
		panic("f may not be nil")
	}

	if events == nil {
		panic("events may not be nil")
	}

	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		logger := telemetryLogger{
			name:   name,
			events: events,
			orig:   findOriginalLogger(ctx),
		}

		telemetryContext := context.WithValue(ctx, job.LoggerKey, &logger)
		interceptContext := &interceptContext{Context: telemetryContext}

		events <- Event{
			Timestamp: time.Now(),
			Type:      EventStart,
			Name:      name,
		}

		newCtx, err := f(interceptContext)
		stop := time.Now()

		if err != nil {
			events <- Event{
				Timestamp: stop,
				Type:      EventFailed,
				Name:      name,
			}
		} else {
			events <- Event{
				Timestamp: stop,
				Type:      EventEnd,
				Name:      name,
			}
		}

		// retarget the interceptContext to remove any instrumentation from the context
		interceptContext.Context = ctx

		return newCtx, err
	})
}

type interceptContext struct {
	context.Context
}

func (ctx *interceptContext) Deadline() (deadline time.Time, ok bool) { return ctx.Context.Deadline() }
func (ctx *interceptContext) Done() <-chan struct{}                   { return ctx.Context.Done() }
func (ctx *interceptContext) Err() error                              { return ctx.Context.Err() }
func (ctx *interceptContext) Value(key interface{}) interface{}       { return ctx.Context.Value(key) }

func findOriginalLogger(ctx context.Context) logger.Logger {
	origLogger, ok := ctx.Value(job.LoggerKey).(logger.Logger)
	for ok {
		var l *telemetryLogger
		l, ok = origLogger.(*telemetryLogger)
		if !ok {
			break
		}

		origLogger = l.orig
	}
	return origLogger
}
