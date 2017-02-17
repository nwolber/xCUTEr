// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"context"
	"log"
	"time"

	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
	"github.com/nwolber/xCUTEr/logger"
)

func instrument(name string, f flunc.Flunc, events *EventStore) flunc.Flunc {
	if name == "" {
		log.Panicln("name may not be empty")
	}

	if f == nil {
		log.Panicln("f may not be nil")
	}

	if events == nil {
		log.Panicln("events may not be nil")
	}

	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		logger := telemetryLogger{
			name:   name,
			events: events,
			orig:   findOriginalLogger(ctx),
		}

		telemetryContext := context.WithValue(ctx, job.LoggerKey, &logger)

		events.store(Event{
			Timestamp: time.Now(),
			Type:      EventStart,
			Name:      name,
		})

		newCtx, err := f(telemetryContext)
		stop := time.Now()

		if err != nil {
			events.store(Event{
				Timestamp: stop,
				Type:      EventFailed,
				Name:      name,
			})
		} else {
			events.store(Event{
				Timestamp: stop,
				Type:      EventEnd,
				Name:      name,
			})
		}

		return newCtx, err
	})
}

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
