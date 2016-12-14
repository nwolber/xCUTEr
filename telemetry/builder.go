// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"sync"
	"time"

	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
)

type nodeGroup struct {
	events *EventStore
	name   string
	group  job.Group
}

func (n *nodeGroup) Append(children ...interface{}) {
	for _, child := range children {
		n.group.Append(child)
	}
}

func (n *nodeGroup) Wrap() interface{} {
	return instrument(n.name, n.group.Wrap().(flunc.Flunc), n.events)
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

type telemetryBuilder struct {
	events *EventStore
	exec   *job.ExecutionTreeBuilder
}

// NewBuilder returns a ConfigBuilder that instruments the
// execution tree for gathering telemetry information.
func NewBuilder() (job.ConfigBuilder, *EventStore) {
	events := &EventStore{}
	return &NamingBuilder{
		NamedConfigBuilder: &telemetryBuilder{
			events: events,
			exec:   &job.ExecutionTreeBuilder{},
		},
	}, events
}

func (t *telemetryBuilder) Sequential(nodeName string) job.Group {
	return &nodeGroup{
		events: t.events,
		name:   nodeName,
		group:  t.exec.Sequential(),
	}
}

func (t *telemetryBuilder) Parallel(nodeName string) job.Group {
	return &nodeGroup{
		events: t.events,
		name:   nodeName,
		group:  t.exec.Parallel(),
	}
}

func (t *telemetryBuilder) Job(nodeName string, name string) job.Group {
	return &nodeGroup{
		events: t.events,
		name:   nodeName,
		group:  t.exec.Job(name),
	}
}

func (t *telemetryBuilder) Output(nodeName string, o *job.Output) interface{} {
	return instrument(nodeName, t.exec.Output(o).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) JobLogger(nodeName string, jobName string) interface{} {
	return instrument(nodeName, t.exec.JobLogger(jobName).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) HostLogger(nodeName string, jobName string, h *job.Host) interface{} {
	return instrument(nodeName, t.exec.HostLogger(jobName, h).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Timeout(nodeName string, timeout time.Duration) interface{} {
	return instrument(nodeName, t.exec.Timeout(timeout).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) SCP(nodeName string, scp *job.ScpData) interface{} {
	return instrument(nodeName, t.exec.SCP(scp).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Hosts(nodeName string) job.Group {
	return &nodeGroup{
		events: t.events,
		name:   nodeName,
		group:  t.exec.Hosts(),
	}
}

func (t *telemetryBuilder) Host(nodeName string, c *job.Config, h *job.Host) job.Group {
	return &nodeGroup{
		events: t.events,
		name:   nodeName,
		group:  t.exec.Host(c, h),
	}
}

func (t *telemetryBuilder) ErrorSafeguard(nodeName string, child interface{}) interface{} {
	return instrument(nodeName, t.exec.ErrorSafeguard(child).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) ContextBounds(nodeName string, child interface{}) interface{} {
	return instrument(nodeName, t.exec.ContextBounds(child).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Retry(nodeName string, child interface{}, retries uint) interface{} {
	return instrument(nodeName, t.exec.Retry(child, retries).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Templating(nodeName string, c *job.Config, h *job.Host) interface{} {
	return instrument(nodeName, t.exec.Templating(c, h).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) SSHClient(nodeName string, host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return instrument(nodeName, t.exec.SSHClient(host, user, keyFile, password, keyboardInteractive).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Forwarding(nodeName string, f *job.Forwarding) interface{} {
	return instrument(nodeName, t.exec.Forwarding(f).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Tunnel(nodeName string, f *job.Forwarding) interface{} {
	return instrument(nodeName, t.exec.Tunnel(f).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Commands(nodeName string, cmd *job.Command) job.Group {
	return &nodeGroup{
		events: t.events,
		name:   nodeName,
		group:  t.exec.Commands(cmd),
	}
}

func (t *telemetryBuilder) Command(nodeName string, cmd *job.Command) interface{} {
	return instrument(nodeName, t.exec.Command(cmd).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) LocalCommand(nodeName string, cmd *job.Command) interface{} {
	return instrument(nodeName, t.exec.LocalCommand(cmd).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Stdout(nodeName string, o *job.Output) interface{} {
	return instrument(nodeName, t.exec.Stdout(o).(flunc.Flunc), t.events)
}

func (t *telemetryBuilder) Stderr(nodeName string, o *job.Output) interface{} {
	return instrument(nodeName, t.exec.Stderr(o).(flunc.Flunc), t.events)
}
