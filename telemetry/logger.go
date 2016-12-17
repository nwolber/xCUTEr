// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/nwolber/xCUTEr/logger"
)

type LogInfo struct {
	File, Message string
	Line          int
}

type telemetryLogger struct {
	orig   logger.Logger
	name   string
	events *EventStore
}

var (
	calldepth = 2
)

func (l *telemetryLogger) Log(t time.Time, message string) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
	}

	_, file = filepath.Split(file)

	if l := len(message); message[l-1] == '\n' {
		message = message[:l-1]
	}

	l.events.store(Event{
		Timestamp: t,
		Type:      EventLog,
		Name:      l.name,
		Info: LogInfo{
			File:    file,
			Line:    line,
			Message: message,
		},
	})
}

func (l *telemetryLogger) SetOutput(w io.Writer) {
	if l.orig != nil {
		l.orig.SetOutput(w)
	}
}

func (l *telemetryLogger) Flags() int {
	if l.orig == nil {
		return 0
	}
	return l.orig.Flags()
}

func (l *telemetryLogger) SetFlags(flag int) {
	if l.orig != nil {
		l.orig.SetFlags(flag)
	}
}

func (l *telemetryLogger) Prefix() string {
	if l.orig == nil {
		return ""
	}
	return l.orig.Prefix()
}

func (l *telemetryLogger) SetPrefix(prefix string) {
	if l.orig != nil {
		l.orig.SetPrefix(prefix)
	}
}

func (l *telemetryLogger) Print(v ...interface{}) {
	s := fmt.Sprint(v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
	}
}

func (l *telemetryLogger) Printf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
	}
}

func (l *telemetryLogger) Println(v ...interface{}) {
	s := fmt.Sprintln(v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
	}
}

func (l *telemetryLogger) Fatal(v ...interface{}) {
	s := fmt.Sprint(v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
		os.Exit(1)
	}
}

func (l *telemetryLogger) Fatalf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
		os.Exit(1)
	}
}

func (l *telemetryLogger) Fatalln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
		os.Exit(1)
	}
}

func (l *telemetryLogger) Panic(v ...interface{}) {
	s := fmt.Sprint(v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
		panic(s)
	}
}

func (l *telemetryLogger) Panicf(format string, v ...interface{}) {
	s := fmt.Sprintf(format, v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
		panic(s)
	}
}

func (l *telemetryLogger) Panicln(v ...interface{}) {
	s := fmt.Sprintln(v...)
	l.Log(time.Now(), s)
	if l.orig != nil {
		l.orig.Output(calldepth, s)
		panic(s)
	}
}

func (l *telemetryLogger) Output(calldepth int, s string) error {
	if l.orig == nil {
		return nil
	}
	return l.orig.Output(calldepth+1, s)
}
