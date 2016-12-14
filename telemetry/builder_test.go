// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
)

func TestEventStore(t *testing.T) {
	s := &EventStore{}
	event := Event{}
	s.store(event)

	expect(t, "store", 1, len(s.events))
	expect(t, "Get", event, s.Get()[0])
	expect(t, "Get", event, s.Reset()[0])
	expect(t, "store", 0, len(s.events))
}

func TestBuilder(t *testing.T) {
	noopFlunc := flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) { return nil, nil })

	builder, _ := NewBuilder()

	_ = builder.Sequential().(*nodeGroup)
	_ = builder.Parallel().(*nodeGroup)
	_ = builder.Job("").(*nodeGroup)
	_ = builder.Output(&job.Output{}).(flunc.Flunc)
	_ = builder.JobLogger("").(flunc.Flunc)
	_ = builder.HostLogger("", &job.Host{}).(flunc.Flunc)
	_ = builder.Timeout(time.Second).(flunc.Flunc)
	_ = builder.SCP(&job.ScpData{}).(flunc.Flunc)
	_ = builder.Hosts().(*nodeGroup)
	_ = builder.Host(&job.Config{}, &job.Host{}).(*nodeGroup)
	_ = builder.ErrorSafeguard(noopFlunc).(flunc.Flunc)
	_ = builder.ContextBounds(noopFlunc).(flunc.Flunc)
	_ = builder.Retry(noopFlunc, 42).(flunc.Flunc)
	_ = builder.Templating(&job.Config{}, &job.Host{}).(flunc.Flunc)
	_ = builder.SSHClient("", "", "", "", nil).(flunc.Flunc)
	_ = builder.Forwarding(&job.Forwarding{}).(flunc.Flunc)
	_ = builder.Tunnel(&job.Forwarding{}).(flunc.Flunc)
	_ = builder.Commands(&job.Command{}).(*nodeGroup)
	_ = builder.Command(&job.Command{}).(flunc.Flunc)
	_ = builder.LocalCommand(&job.Command{}).(flunc.Flunc)
	_ = builder.Stdout(&job.Output{}).(flunc.Flunc)
	_ = builder.Stderr(&job.Output{}).(flunc.Flunc)
}
