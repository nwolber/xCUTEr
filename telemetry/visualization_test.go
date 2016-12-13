// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"testing"
	"time"

	"github.com/nwolber/xCUTEr/job"
)

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
