// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"context"
	"fmt"
	"io"
	"log"
	"runtime"
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
	Log(t time.Time, file string, line int, message string)
	stringer
}

// A LogMessage .
type LogMessage struct {
	Timestamp     time.Time
	Line          int
	File, Message string
}

type simpleNode struct {
	typ    string
	exec   flunc.Flunc
	str    stringer
	status NodeStatus
	text   string
	log    []*LogMessage
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

func (n *simpleNode) Log(t time.Time, file string, line int, message string) {
	n.log = append(n.log, &LogMessage{
		Timestamp: t,
		File:      file,
		Line:      line,
		Message:   message,
	})
}

var (
	yellow = color.New(color.FgYellow).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
)

func formatString(str string, status NodeStatus, text string) string {
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

	if text != "" {
		str += ": " + text
	}

	if color != nil {
		index := strings.Index(str, "\n")
		if index >= 0 {
			str = color(str[:index]) + str[index:]
		} else {
			str = color(str)
		}
	}

	return str
}

func (n *simpleNode) String(v *vars) string {
	str := ""
	if n.v != nil {
		str = n.str.String(n.v)
	} else {
		str = n.str.String(v)
	}

	text := n.text
	if text == "" && len(n.log) > 0 {
		text = "\n"
		for _, msg := range n.log {
			text += fmt.Sprintf("%s %s:%d %s\n", msg.Timestamp, msg.File, msg.Line, msg.Message)
		}
	}

	return formatString(str, n.status, text)
}

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
}

type interceptLogger struct {
	l, orig Logger
	log     func(t time.Time, file string, line int, message string)
}

func (l *interceptLogger) Log(t time.Time, message string) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
	}

	if l := len(message); message[l-1] == '\n' {
		message = message[:l-1]
	}

	l.log(t, file, line, message)
}

func (l *interceptLogger) SetOutput(w io.Writer) {
	if l.orig != nil {
		l.orig.SetOutput(w)
	}
}

func (l *interceptLogger) Flags() int {
	if l.orig == nil {
		return 0
	}
	return l.orig.Flags()
}

func (l *interceptLogger) SetFlags(flag int) {
	if l.orig != nil {
		l.orig.SetFlags(flag)
	}
}

func (l *interceptLogger) Prefix() string {
	if l.orig == nil {
		return ""
	}
	return l.orig.Prefix()
}

func (l *interceptLogger) SetPrefix(prefix string) {
	if l.orig != nil {
		l.orig.SetPrefix(prefix)
	}
}

func (l *interceptLogger) Print(v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintln(v...))
	if l.orig != nil {
		l.orig.Print(v...)
	}
}

func (l *interceptLogger) Printf(format string, v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintf(format, v...))
	if l.orig != nil {
		l.orig.Printf(format, v...)
	}
}

func (l *interceptLogger) Println(v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintln(v...))
	if l.orig != nil {
		l.orig.Println(v...)
	}
}

func (l *interceptLogger) Fatal(v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintln(v...))
	if l.orig != nil {
		l.orig.Fatal(v...)
	}
}

func (l *interceptLogger) Fatalf(format string, v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintf(format, v...))
	if l.orig != nil {
		l.orig.Fatalf(format, v...)
	}
}

func (l *interceptLogger) Fatalln(v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintln(v...))
	if l.orig != nil {
		l.orig.Fatalln(v...)
	}
}

func (l *interceptLogger) Panic(v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintln(v...))
	if l.orig != nil {
		l.orig.Panic(v...)
	}
}

func (l *interceptLogger) Panicf(format string, v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintf(format, v...))
	if l.orig != nil {
		l.orig.Panicf(format, v...)
	}
}

func (l *interceptLogger) Panicln(v ...interface{}) {
	l.Log(time.Now(), fmt.Sprintln(v...))
	if l.orig != nil {
		l.orig.Panicln(v...)
	}
}

func (l *interceptLogger) Output(calldepth int, s string) error {
	if l.orig == nil {
		return nil
	}
	return l.orig.Output(calldepth, s)
}

type interceptContext struct {
	context.Context
}

func (ctx *interceptContext) Deadline() (deadline time.Time, ok bool) { return ctx.Context.Deadline() }
func (ctx *interceptContext) Done() <-chan struct{}                   { return ctx.Context.Done() }
func (ctx *interceptContext) Err() error                              { return ctx.Context.Err() }
func (ctx *interceptContext) Value(key interface{}) interface{}       { return ctx.Context.Value(key) }

func instrument(update func(NodeStatus, string, string), n node, f flunc.Flunc) flunc.Flunc {
	return flunc.MakeFlunc(func(origCtx context.Context) (context.Context, error) {
		tt, _ := origCtx.Value(templatingKey).(*templatingEngine)
		if tt != nil {
			switch nn := n.(type) {
			case *simpleNode:
				nn.v = &vars{tt: tt}
			case *nodeGroup:
				nn.v = &vars{tt: tt}
			}
		}

		origLogger, ok := origCtx.Value(LoggerKey).(Logger)
		for ok {
			var l *interceptLogger
			l, ok = origLogger.(*interceptLogger)
			if !ok {
				break
			}

			origLogger = l.orig
		}

		ctx := context.WithValue(origCtx, LoggerKey, &interceptLogger{orig: origLogger, log: n.Log})
		interceptCtx := &interceptContext{Context: ctx}

		n.Status(RunningNode, "")
		update(RunningNode, n.Typ(), "")

		newCtx, err := f(interceptCtx)

		if err == nil {
			n.Status(CompletedNode, "")
			update(CompletedNode, n.Typ(), "")
		} else {
			n.Status(FailedNode, err.Error())
			update(FailedNode, n.Typ(), err.Error())
		}

		// The interceptContext is now the parent of all modifications to the context
		// made by f. By retargeting the incerceptContext at the original context,
		// we effectivly remove the interceptLogger from the context
		interceptCtx.Context = origCtx

		return newCtx, err
	})
}

type stringerGroup interface {
	Group
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
	v        *vars
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

func (*nodeGroup) Log(t time.Time, file string, line int, message string) {}

func (g *nodeGroup) Exec() flunc.Flunc {
	return instrument(g.update, g, g.exec.Wrap().(flunc.Flunc))
}

func (g *nodeGroup) String(v *vars) string {
	return formatString(g.str.String(g.v), g.status, g.text)
}

type telemetryBuilder struct {
	u    chan NodeEvent
	str  *stringVisitor
	exec ExecutionTreeVisitor
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

func (t *telemetryBuilder) Sequential() Group {
	var g nodeGroup
	g.typ = "Sequential"
	g.update = t.update
	g.exec = t.exec.Sequential().(*executionGroup)
	g.str = t.str.Sequential().(*multiple)
	return &g
}

func (t *telemetryBuilder) Parallel() Group {
	var g nodeGroup
	g.typ = "Parallel"
	g.update = t.update
	g.exec = t.exec.Parallel().(*executionGroup)
	g.str = t.str.Parallel().(*multiple)
	return &g
}

func (t *telemetryBuilder) Job(name string) Group {
	var g nodeGroup
	g.update = t.update
	g.exec = t.exec.Job(name).(*executionGroup)
	str := t.str.Job(name).(*multiple)
	g.str = str
	g.typ = str.typ
	return &g
}

func (t *telemetryBuilder) Output(o *Output) interface{} {
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

func (t *telemetryBuilder) HostLogger(jobName string, h *Host) interface{} {
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

func (t *telemetryBuilder) SCP(scp *ScpData) interface{} {
	var n simpleNode
	n.typ = "SCP"
	n.str = t.str.SCP(scp).(stringer)
	n.exec = instrument(t.update, &n, t.exec.SCP(scp).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Hosts() Group {
	var g nodeGroup
	g.typ = "Target hosts"
	g.update = t.update
	g.exec = t.exec.Hosts().(*executionGroup)
	g.str = t.str.Hosts().(*multiple)
	return &g
}

func (t *telemetryBuilder) Host(c *Config, h *Host) Group {
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
	n.exec = instrument(t.update, &n, t.exec.Retry(child.(node).Exec(), retries).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Templating(c *Config, h *Host) interface{} {
	var n simpleNode
	n.typ = "Templating"
	n.str = t.str.Templating(c, h).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Templating(c, h).(flunc.Flunc))

	return &n
}

func (t *telemetryBuilder) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("SSH client %s@%s", user, host)
	n.str = t.str.SSHClient(host, user, keyFile, password, keyboardInteractive).(stringer)
	n.exec = instrument(t.update, &n, t.exec.SSHClient(host, user, keyFile, password, keyboardInteractive).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Forwarding(f *Forwarding) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Forward %s:%d to %s:%d", f.RemoteHost, f.RemotePort, f.LocalHost, f.LocalPort)
	n.str = t.str.Forwarding(f).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Forwarding(f).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Tunnel(f *Forwarding) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Tunnel %s:%d to %s:%d", f.LocalHost, f.LocalPort, f.RemoteHost, f.RemotePort)
	n.str = t.str.Tunnel(f).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Tunnel(f).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Commands(cmd *Command) Group {
	var g nodeGroup
	g.typ = "Command"
	g.update = t.update
	g.exec = t.exec.Commands(cmd).(*executionGroup)
	g.str = t.str.Commands(cmd).(*multiple)
	return &g
}

func (t *telemetryBuilder) Command(cmd *Command) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Command %q", cmd.Command)
	n.str = t.str.Command(cmd).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Command(cmd).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) LocalCommand(cmd *Command) interface{} {
	var n simpleNode
	n.typ = fmt.Sprintf("Local Command %q", cmd.Command)
	n.str = t.str.LocalCommand(cmd).(stringer)
	n.exec = instrument(t.update, &n, t.exec.LocalCommand(cmd).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Stdout(o *Output) interface{} {
	var n simpleNode
	n.typ = "Stdout"
	n.str = t.str.Stdout(o).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Stdout(o).(flunc.Flunc))
	return &n
}

func (t *telemetryBuilder) Stderr(o *Output) interface{} {
	var n simpleNode
	n.typ = "Stderr"
	n.str = t.str.Stderr(o).(stringer)
	n.exec = instrument(t.update, &n, t.exec.Stderr(o).(flunc.Flunc))
	return &n
}
