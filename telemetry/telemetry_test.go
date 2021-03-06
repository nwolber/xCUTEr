// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
	"github.com/nwolber/xCUTEr/logger"
)

func expect(t *testing.T, name string, want, got interface{}) bool {
	if want != got {
		if _, ok := want.(string); ok {
			t.Errorf("%s: want: %q, got %q", name, want, got)
		} else {
			t.Errorf("%s: want: %+v, got %+v", name, want, got)
		}
		return false
	}
	return true
}

func expectEvents(t *testing.T, name string, want, got []Event) bool {
	if !expect(t, name, len(want), len(got)) {
		return false
	}

	for i, wantEvent := range want {
		gotEvent := got[i]

		if (gotEvent.Timestamp == time.Time{}) {
			t.Errorf("%s pos: %d, Timestamp is zero", name, i)
		}

		if wantEvent.Name != gotEvent.Name {
			t.Errorf("%s pos: %d, want name: %s, got: %s", name, i, wantEvent.Name, gotEvent.Name)
		}

		if wantEvent.Type != gotEvent.Type {
			t.Errorf("%s pos: %d, want type: %s, got: %s", name, i, wantEvent.Type, gotEvent.Type)
		}

		if wantEvent.Info.Message != gotEvent.Info.Message {
			t.Errorf("%s pos: %d, want log message: '%s', got: '%s'", name, i, wantEvent.Info.Message, gotEvent.Info.Message)
		}
	}

	return true
}

func discardEvents(events <-chan Event) {
	for range events {
	}
}

func TestInstrument(t *testing.T) {
	const logText = "test log message"

	var (
		errFuncFailed = errors.New("failingFlunc failed")

		noopFlunc = flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
			return nil, nil
		})

		failingFlunc = flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
			return nil, errFuncFailed
		})

		loggingFlunc = flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
			logger := ctx.Value(job.LoggerKey).(logger.Logger)

			logger.Print(logText)

			return nil, nil
		})
	)

	tests := []struct {
		name string
		f    flunc.Flunc
		want []Event
		err  error
	}{
		{
			name: "noop",
			f:    noopFlunc,
			want: []Event{
				{Name: "noop", Type: EventStart},
				{Name: "noop", Type: EventEnd},
			},
		},
		{
			name: "failing",
			f:    failingFlunc,
			want: []Event{
				{Name: "failing", Type: EventStart},
				{Name: "failing", Type: EventFailed},
			},
			err: errFuncFailed,
		},
		{
			name: "logging",
			f:    loggingFlunc,
			want: []Event{
				{Name: "logging", Type: EventStart},
				{Name: "logging", Type: EventLog, Info: LogInfo{Message: logText}},
				{Name: "logging", Type: EventEnd},
			},
		},
	}

	for _, test := range tests {
		events := &EventStore{}

		f := instrument(test.name, test.f, events)

		expectEvents(t, test.name, []Event{}, events.events)

		_, err := f(context.TODO())

		expect(t, test.name, test.err, err)
		expectEvents(t, test.name, test.want, events.events)
	}
}

func TestInstrumentWithAlteredContext(t *testing.T) {
	// Fluncs may return a new context to store new values.
	// Instrument has to make sure, that this new context is
	// passed correctly to nested fluncs. It also has to make
	// sure to remove any objects stored in the context by itself
	// to be removed before the context is passed on.
	const (
		key   = "super special key used only in testing"
		value = 42
	)

	alteringFlunc := flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		return context.WithValue(ctx, key, value), nil
	})

	f := instrument("bla", alteringFlunc, &EventStore{})
	ctx := context.Background()
	returnedCtx, err := f(ctx)
	expect(t, "returned value", value, returnedCtx.Value(key))
	if returnedCtx.Value(job.LoggerKey) == nil {
		t.Error("want a logger, got nil")
	}
	expect(t, "error", nil, err)

}

func TestFindOriginalLogger(t *testing.T) {
	l := logger.New(&log.Logger{}, false)
	origCtx := context.WithValue(context.Background(), job.LoggerKey, l)
	ctx := context.WithValue(origCtx, job.LoggerKey, &telemetryLogger{orig: l})

	got := findOriginalLogger(ctx)

	expect(t, "", l, got)
}
