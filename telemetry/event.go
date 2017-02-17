// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"sync"
	"time"
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

// An Event is created, when a flunc starts, ends or generates a log message.
type Event struct {
	Type      EventType
	Timestamp time.Time
	Name      string
	Info      LogInfo
}

// An EventStore stores Events.
type EventStore struct {
	m      sync.Mutex
	events []Event
}

func (e *EventStore) store(event Event) {
	e.m.Lock()
	defer e.m.Unlock()
	e.events = append(e.events, event)
}

// Get returns a copy of all Events in the store.
func (e *EventStore) Get() []Event {
	e.m.Lock()
	defer e.m.Unlock()
	events := e.events
	return events
}

// Reset clears all events and returns a copy of all
// events that have been cleared.
func (e *EventStore) Reset() []Event {
	e.m.Lock()
	defer e.m.Unlock()
	events := e.events
	e.events = []Event{}
	return events
}
