// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nwolber/xCUTEr/flunc"
)

// TelemetryInfo holds informations about an instrumented job
type TelemetryInfo struct {
	updateTrigger chan NodeEvent
	root          node
	events        []NodeEvent
}

// Events returns a list of event that occured during execution
func (t *TelemetryInfo) Events() NodeEvents {
	return t.events
}

// GetFlunc executes the instrumented job, the channel must be serviced, otherwise execution hangs.
func (t *TelemetryInfo) GetFlunc() (flunc.Flunc, <-chan string) {
	update := make(chan string)
	go func(n node, updateTrigger <-chan NodeEvent, update chan<- string) {
		for {
			event, ok := <-updateTrigger
			if !ok {
				close(update)
				return
			}

			t.events = append(t.events, event)

			update <- n.String(nil)
		}
	}(t.root, t.updateTrigger, update)

	f := func(ctx context.Context) (context.Context, error) {
		t.root.Status(NotStartedNode, "")
		t.events = make([]NodeEvent, 0, len(t.events))
		newCtx, err := t.root.Exec()(ctx)
		close(t.updateTrigger)
		return newCtx, err
	}

	return f, update
}

// Instrument the given job.
func Instrument(c *Config) (*TelemetryInfo, error) {
	t := newTelemtryBuilder()
	ret, err := visitConfig(t, c)
	if err != nil {
		return nil, err
	}

	return &TelemetryInfo{
		updateTrigger: t.u,
		root:          ret.(node),
	}, nil
}

// NodeStatus describes the status of a node.
type NodeStatus int

const (
	// Execution didn't start yet.
	NotStartedNode NodeStatus = iota
	// Node is currently executed.
	RunningNode
	// Execution completed.
	CompletedNode
	// Execution failed.
	FailedNode
)

// NodeEvent describes a event during execution
type NodeEvent struct {
	Timestamp time.Time
	Status    NodeStatus
	Typ, Text string
}

// A EventFilter returns wether the e matches the filter.
type EventFilter func(e NodeEvent) bool

// NodeEvents is a list of events
type NodeEvents []NodeEvent

// Filter returns all NodeEvents matching f.
func (e *NodeEvents) Filter(f EventFilter) NodeEvents {
	var arr NodeEvents
	for _, event := range *e {
		if f(event) {
			arr = append(arr, event)
		}
	}
	return arr
}

// FilterType returns all NodeEvents with type typ.
func (e *NodeEvents) FilterType(typ string) NodeEvents {
	return e.Filter(func(e NodeEvent) bool {
		return e.Typ == typ
	})
}

// FilterStatus returns all NodeEvents with status status.
func (e *NodeEvents) FilterStatus(status NodeStatus) NodeEvents {
	return e.Filter(func(e NodeEvent) bool {
		return e.Status == status
	})
}

type node interface {
	Typ() string
	Status(status NodeStatus, text string)
	Exec() flunc.Flunc
	stringer
}

type simpleNode struct {
	typ    string
	exec   flunc.Flunc
	str    stringer
	status NodeStatus
	text   string
	v      *vars
}

func (n *simpleNode) Typ() string {
	return n.typ
}

func (n *simpleNode) Status(status NodeStatus, text string) {
	n.status = status
	n.text = text
}

func (n *simpleNode) Exec() flunc.Flunc {
	return n.exec
}

var (
	yellow = color.New(color.FgYellow).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
)

func formatString(str string, status NodeStatus, text string) string {
	if text != "" {
		text = ": " + text
	}

	var color func(a ...interface{}) string
	switch status {
	case RunningNode:
		str = "➤ " + str
		color = yellow
	case CompletedNode:
		str = "✔ " + str
		color = green
	case FailedNode:
		str = "✘ " + str
		color = red
	}

	if color != nil {
		index := strings.Index(str, "\n")
		if index >= 0 {
			str = color(str[:index]+text) + str[index:]
		} else {
			str = color(str + text)
		}
	}

	return str
}

func (n *simpleNode) String(v *vars) string {
	str := ""
	if n.v != nil {
		str = n.str.String(v)
	} else {
		str = n.str.String(v)
	}

	return formatString(str, n.status, n.text)
}

func instrument(update func(NodeStatus, string, string), n node, f flunc.Flunc) flunc.Flunc {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		n.Status(RunningNode, "")
		update(RunningNode, n.Typ(), "")
		newCtx, err := f(ctx)
		if err == nil {
			n.Status(CompletedNode, "")
			update(CompletedNode, n.Typ(), "")
		} else {
			n.Status(FailedNode, err.Error())
			update(FailedNode, n.Typ(), err.Error())
		}

		return newCtx, err
	})
}

type stringerGroup interface {
	group
	stringer
}

type nodeGroup struct {
	typ      string
	children []node
	status   NodeStatus
	text     string
	exec     *executionGroup
	str      stringerGroup
	update   func(NodeStatus, string, string)
}

func (g *nodeGroup) Append(children ...interface{}) {
	for _, child := range children {
		if child == nil {
			continue
		}

		n, ok := child.(node)
		if !ok {
			log.Panicf("not a node %T", child)
		}

		g.exec.Append(n.Exec())
		g.str.Append(n.(stringer))
		g.children = append(g.children, n)
	}
}

func (g *nodeGroup) Wrap() interface{} {
	return g
}

func (g *nodeGroup) Typ() string {
	return g.typ
}

func (g *nodeGroup) Status(status NodeStatus, text string) {
	g.status = status
	g.text = text

	if status == NotStartedNode {
		for _, child := range g.children {
			child.Status(status, text)
		}
	}
}

func (g *nodeGroup) Exec() flunc.Flunc {
	return instrument(g.update, g, g.exec.Wrap().(flunc.Flunc))
}

func (g *nodeGroup) String(v *vars) string {
	return formatString(g.str.String(v), g.status, g.text)
}

type telemetryBuilder struct {
	u    chan NodeEvent
	str  *stringVisitor
	exec executionTreeVisitor
}

func newTelemtryBuilder() *telemetryBuilder {
	return &telemetryBuilder{
		u: make(chan NodeEvent),
		str: &stringVisitor{
			full: true,
		},
	}
}

func (t *telemetryBuilder) update(status NodeStatus, typ, text string) {
	t.u <- NodeEvent{
		Timestamp: time.Now(),
		Status:    status,
		Typ:       typ,
		Text:      text,
	}
}

func (t *telemetryBuilder) Sequential() group {
	var g nodeGroup
	g.typ = "Sequential"
	g.update = t.update
	g.exec = t.exec.Sequential().(*executionGroup)
	g.str = t.str.Sequential().(*multiple)
	return &g
}

func (t *telemetryBuilder) Parallel() group {
	var g nodeGroup
	g.typ = "Parallel"
	g.update = t.update
	g.exec = t.exec.Parallel().(*executionGroup)
	g.str = t.str.Parallel().(*multiple)
	return &g
}

func (t *telemetryBuilder) Job(name string) group {
	var g nodeGroup
	g.update = t.update
	g.exec = t.exec.Job(name).(*executionGroup)
	str := t.str.Job(name).(*multiple)
	g.str = str
	g.typ = str.typ
	return &g
}

func (t *telemetryBuilder) Output(o *output) interface{} {
	if o == nil {
		return nil
	}

	var n simpleNode
	n.typ = "Output"
	n.str = t.str.Output(o).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Output(o).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) JobLogger(jobName string) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Job %s", jobName)
	n.str = t.str.JobLogger(jobName).(stringer)
	n.exec = instrument(t.update, &n, t.exec.JobLogger(jobName).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) HostLogger(jobName string, h *host) interface{} {
	var n simpleNode
	n.typ = "Host logger"
	n.str = t.str.HostLogger(jobName, h).(stringer)
	n.exec = instrument(t.update, &n, t.exec.HostLogger(jobName, h).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Timeout(timeout time.Duration) interface{} {
	var n simpleNode
	n.typ = "Timeout"
	n.str = t.str.Timeout(timeout).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Timeout(timeout).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) SCP(scp *scpData) interface{} {
	var n simpleNode
	n.typ = "SCP"
	n.str = t.str.SCP(scp).(stringer)
	n.exec = instrument(t.update, &n, t.exec.SCP(scp).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Hosts() group {
	var g nodeGroup
	g.typ = "Target hosts"
	g.update = t.update
	g.exec = t.exec.Hosts().(*executionGroup)
	g.str = t.str.Hosts().(*multiple)
	return &g
}

func (t *telemetryBuilder) Host(c *Config, h *host) group {
	var g nodeGroup
	g.update = t.update
	g.exec = t.exec.Host(c, h).(*executionGroup)
	str := t.str.Host(c, h).(*partHost)
	g.str = str
	g.typ = str.typ
	return &g
}

func (t *telemetryBuilder) ErrorSafeguard(child interface{}) interface{} {
	var n simpleNode
	n.typ = "Error Safeguard"
	n.str = t.str.ErrorSafeguard(child).(stringer)
	n.exec = instrument(t.update, &n, t.exec.ErrorSafeguard(child.(node).Exec()).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) ContextBounds(child interface{}) interface{} {
	var n simpleNode
	n.typ = "Context Bounds"
	n.str = t.str.ContextBounds(child).(stringer)
	n.exec = instrument(t.update, &n, t.exec.ContextBounds(child.(node).Exec()).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Retry(child interface{}, retries uint) interface{} {
	var n simpleNode
	n.typ = "Retry"
	n.str = t.str.Retry(child, retries).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Retry(child, retries).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Templating(c *Config, h *host) interface{} {
	var n simpleNode
	n.typ = "Templating"
	n.str = t.str.Templating(c, h).(stringer)

	f := t.exec.Templating(c, h).(flunc.Flunc)
	wrapper := func(ctx context.Context) (context.Context, error) {
		newCtx, err := f(ctx)
		n.v = &vars{
			tt: newCtx.Value(templatingKey).(*templatingEngine),
		}
		return newCtx, err
	}
	n.exec = instrument(t.update, &n, wrapper)
	return &n
}

func (t *telemetryBuilder) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("SSH client %s@%s", user, host)
	n.str = t.str.SSHClient(host, user, keyFile, password, keyboardInteractive).(stringer)
	n.exec = instrument(t.update, &n, t.exec.SSHClient(host, user, keyFile, password, keyboardInteractive).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Forwarding(f *forwarding) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Forward %s:%d to %s:%d", f.RemoteHost, f.RemotePort, f.LocalHost, f.LocalPort)
	n.str = t.str.Forwarding(f).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Forwarding(f).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Tunnel(f *forwarding) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Tunnel %s:%d to %s:%d", f.LocalHost, f.LocalPort, f.RemoteHost, f.RemotePort)
	n.str = t.str.Tunnel(f).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Tunnel(f).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Commands(cmd *command) group {
	var g nodeGroup
	g.typ = "Command"
	g.update = t.update
	g.exec = t.exec.Commands(cmd).(*executionGroup)
	g.str = t.str.Commands(cmd).(*multiple)
	return &g
}

func (t *telemetryBuilder) Command(cmd *command) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Command %q", cmd.Command)
	n.str = t.str.Command(cmd).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Command(cmd).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) LocalCommand(cmd *command) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Local Command %q", cmd.Command)
	n.str = t.str.LocalCommand(cmd).(stringer)
	n.exec = instrument(t.update, &n, t.exec.LocalCommand(cmd).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Stdout(o *output) interface{} {
	var n simpleNode
	n.typ = "Stdout"
	n.str = t.str.Stdout(o).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Stdout(o).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Stderr(o *output) interface{} {
	var n simpleNode
	n.typ = "Stderr"
	n.str = t.str.Stderr(o).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Stderr(o).(flunc.Flunc))
	return &n
}
