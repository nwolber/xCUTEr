// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"testing"
	"time"

	"github.com/nwolber/xCUTEr/job"
)

func TestStringBuilder(t *testing.T) {
	stringer := job.Leaf("")

	builder := newStringBuilder()
	builder.(*NamingBuilder).NamedConfigBuilder.(*stringBuilder).str.Full = true

	_ = builder.Sequential().(*visualizationNode)
	_ = builder.Parallel().(*visualizationNode)
	_ = builder.Job("").(*visualizationNode)
	_ = builder.Output(&job.Output{}).(*visualizationNode)
	_ = builder.JobLogger("").(*visualizationNode)
	_ = builder.HostLogger("", &job.Host{}).(*visualizationNode)
	_ = builder.Timeout(time.Second).(*visualizationNode)
	_ = builder.SCP(&job.ScpData{}).(*visualizationNode)
	_ = builder.Hosts().(*visualizationNode)
	_ = builder.Host(&job.Config{}, &job.Host{}).(*visualizationNode)
	_ = builder.ErrorSafeguard(stringer).(*visualizationNode)
	_ = builder.ContextBounds(stringer).(*visualizationNode)
	_ = builder.Retry(stringer, 42).(*visualizationNode)
	_ = builder.Templating(&job.Config{}, &job.Host{}).(*visualizationNode)
	_ = builder.SSHClient("", "", "", "", nil).(*visualizationNode)
	_ = builder.Forwarding(&job.Forwarding{}).(*visualizationNode)
	_ = builder.Tunnel(&job.Forwarding{}).(*visualizationNode)
	_ = builder.Commands(&job.Command{}).(*visualizationNode)
	_ = builder.Command(&job.Command{}).(*visualizationNode)
	_ = builder.LocalCommand(&job.Command{}).(*visualizationNode)
	_ = builder.Stdout(&job.Output{}).(*visualizationNode)
	_ = builder.Stderr(&job.Output{}).(*visualizationNode)
}

func TestSequential(t *testing.T) {
	var want = green("✔ Sequential")

	builder := newStringBuilder()
	ret := builder.Sequential()

	node, ok := ret.(*visualizationNode)
	if !ok {
		t.Fatalf("expected a *visualizationNode, got %T", ret)
	}

	node.Status = StateCompleted
	got := node.String(&job.Vars{})
	expect(t, "TestSequential", want, got)
}

func TestTimeout(t *testing.T) {
	var want = green("✔ Timeout: 1m0s")

	builder := newStringBuilder()
	ret := builder.Timeout(time.Minute)

	node, ok := ret.(*visualizationNode)
	if !ok {
		t.Fatalf("expected a *visualizationNode, got %T", ret)
	}

	node.Status = StateCompleted
	got := node.String(&job.Vars{})
	expect(t, "TestTimeout", want, got)
}

func TestTree(t *testing.T) {
	want :=
		green("✔ Sequential") + "\n" +
			"└─ " + green("✔ Timeout: 1m0s")

	builder := newStringBuilder()

	ret := builder.Sequential()
	seq := ret.(*visualizationNode)
	seq.Status = StateCompleted

	ret2 := builder.Timeout(time.Minute)
	timeout := ret2.(*visualizationNode)
	timeout.Status = StateCompleted
	seq.Append(timeout)

	got := seq.Wrap().(*visualizationNode).String(&job.Vars{})
	expect(t, "", want, got)
}
