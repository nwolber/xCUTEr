// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/nwolber/xCUTEr/job"
)

// Visualization shows the execution tree of a job
// in a specific state.
type Visualization struct {
	root  *visualizationNode
	nodes map[string]*visualizationNode
}

// NewVisualization generates a new Visualization from the given config.
func NewVisualization(c *job.Config) (*Visualization, error) {
	builder := newStringBuilder()
	tree, err := job.VisitConfig(builder, c)
	if err != nil {
		return nil, err
	}

	return &Visualization{
		root:  tree.(*visualizationNode),
		nodes: builder.(*NamingBuilder).NamedConfigBuilder.(*stringBuilder).nodes,
	}, nil
}

// ApplyChan progressivly applies the job events sent
// through the channel to the visualization.
func (v *Visualization) ApplyChan(events <-chan Event) {
	for event := range events {
		v.Apply(event)
	}
}

// ApplyStore applies all events to the visualization.
func (v *Visualization) ApplyStore(events []Event) {
	for _, event := range events {
		v.Apply(event)
	}
}

// Apply applies an event to the visualization.
func (v *Visualization) Apply(event Event) {
	node, ok := v.nodes[event.Name]
	if !ok {
		return
	}

	switch event.Type {
	case EventStart:
		node.Status = StateRunning
	case EventEnd:
		node.Status = StateCompleted
	case EventFailed:
		node.Status = StateFailed
	case EventLog:
		info := event.Info.(LogInfo)
		node.Append(job.Leaf(fmt.Sprintf("%s %s:%d: %s", event.Timestamp, info.File, info.Line, info.Message)))
	}
}

func (v *Visualization) String() string {
	return v.root.String(&job.Vars{})
}

// NodeStatus describes the status of a node.
type NodeStatus int

const (
	// Execution didn't start yet.
	StateReady NodeStatus = iota
	// Node is currently executed.
	StateRunning
	// Execution completed.
	StateCompleted
	// Execution failed.
	StateFailed
)

type visualizationNode struct {
	Status NodeStatus
	job.Branch
}

func (s *visualizationNode) Wrap() interface{} {
	return s
}

func (s *visualizationNode) String(v *job.Vars) string {
	return formatString(s.Branch.String(v), s.Status)
}

var (
	yellow = color.New(color.FgYellow).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
)

func formatString(str string, status NodeStatus) string {
	var color func(a ...interface{}) string
	switch status {
	case StateRunning:
		str = "➤ " + str
		color = yellow
	case StateCompleted:
		str = "✔ " + str
		color = green
	case StateFailed:
		str = "✘ " + str
		color = red
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

type stringBuilder struct {
	str   *job.StringBuilder
	nodes map[string]*visualizationNode
}

func (t *stringBuilder) storeNode(nodeName string, node *visualizationNode) *visualizationNode {
	t.nodes[nodeName] = node
	return node
}

func newStringBuilder() job.ConfigBuilder {
	return &NamingBuilder{
		NamedConfigBuilder: &stringBuilder{
			str:   &job.StringBuilder{Full: true},
			nodes: make(map[string]*visualizationNode),
		},
	}
}

func (t *stringBuilder) Sequential(nodeName string) job.Group {
	return t.storeNode(nodeName, &visualizationNode{Branch: t.str.Sequential().(job.Branch)})
}

func (t *stringBuilder) Parallel(nodeName string) job.Group {
	return t.storeNode(nodeName, &visualizationNode{Branch: t.str.Parallel().(job.Branch)})
}

func (t *stringBuilder) Job(nodeName string, name string) job.Group {
	return t.storeNode(nodeName, &visualizationNode{Branch: t.str.Job(name).(job.Branch)})
}

func (t *stringBuilder) Output(nodeName string, o *job.Output) interface{} {
	if root := t.str.Output(o); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) JobLogger(nodeName string, jobName string) interface{} {
	if root := t.str.JobLogger(jobName); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) HostLogger(nodeName string, jobName string, h *job.Host) interface{} {
	if root := t.str.HostLogger(jobName, h); root != nil {

		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) Timeout(nodeName string, timeout time.Duration) interface{} {
	if root := t.str.Timeout(timeout); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) SCP(nodeName string, scp *job.ScpData) interface{} {
	if root := t.str.SCP(scp); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) Hosts(nodeName string) job.Group {
	if root := t.str.Hosts(); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: root.(job.Branch)})
	}

	return nil
}

func (t *stringBuilder) Host(nodeName string, c *job.Config, h *job.Host) job.Group {
	if root := t.str.Host(c, h); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: root.(job.Branch)})
	}

	return nil
}

func (t *stringBuilder) ErrorSafeguard(nodeName string, child interface{}) interface{} {
	return t.storeNode(nodeName, &visualizationNode{Branch: t.str.ErrorSafeguard(child).(job.Branch)})
}

func (t *stringBuilder) ContextBounds(nodeName string, child interface{}) interface{} {
	return t.storeNode(nodeName, &visualizationNode{Branch: t.str.ContextBounds(child).(job.Branch)})
}

func (t *stringBuilder) Retry(nodeName string, child interface{}, retries uint) interface{} {
	if root := t.str.Retry(child, retries); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: root.(job.Branch)})
	}

	return nil
}

func (t *stringBuilder) Templating(nodeName string, c *job.Config, h *job.Host) interface{} {
	if root := t.str.Templating(c, h); root != nil {
		t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) SSHClient(nodeName string, host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	if root := t.str.SSHClient(host, user, keyFile, password, keyboardInteractive); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) Forwarding(nodeName string, f *job.Forwarding) interface{} {
	if root := t.str.Forwarding(f); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) Tunnel(nodeName string, f *job.Forwarding) interface{} {
	if root := t.str.Tunnel(f); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) Commands(nodeName string, cmd *job.Command) job.Group {
	return t.storeNode(nodeName, &visualizationNode{Branch: t.str.Commands(cmd).(job.Branch)})
}

func (t *stringBuilder) Command(nodeName string, cmd *job.Command) interface{} {
	if root := t.str.Command(cmd); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) LocalCommand(nodeName string, cmd *job.Command) interface{} {
	if root := t.str.LocalCommand(cmd); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) Stdout(nodeName string, o *job.Output) interface{} {
	if root := t.str.Stdout(o); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}

func (t *stringBuilder) Stderr(nodeName string, o *job.Output) interface{} {
	if root := t.str.Stderr(o); root != nil {
		return t.storeNode(nodeName, &visualizationNode{Branch: &job.SimpleBranch{Root: root.(job.Leaf)}})
	}

	return nil
}
