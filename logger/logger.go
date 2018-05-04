// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package logger

import (
	"fmt"
	"io"
	"log"
)

// Logger is an interface for loggers. It is satisfied by log.Logger.
type Logger interface {
	SetOutput(w io.Writer)
	Flags() int
	SetFlags(flag int)
	Prefix() string
	SetPrefix(prefix string)
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
	Panicln(v ...interface{})
	Output(calldepth int, s string) error
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
}

type errorLogger struct {
	*log.Logger
	printStack bool
}

func New(logger *log.Logger, printStack bool) Logger {
	return &errorLogger{
		Logger:     logger,
		printStack: printStack,
	}
}

func (l *errorLogger) Error(v ...interface{}) {
	if l.printStack {
		for i, vv := range v {
			if err, ok := vv.(error); ok {
				v[i] = fmt.Sprintf("%+v", err)
			}
		}
	}

	l.Logger.Println(v...)
}

func (l *errorLogger) Errorf(format string, v ...interface{}) {
	if l.printStack {
		for i, vv := range v {
			if err, ok := vv.(error); ok {
				v[i] = fmt.Sprintf("%+v", err)
			}
		}
	}

	l.Logger.Printf(format, v...)
}
