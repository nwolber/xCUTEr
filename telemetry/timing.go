// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"time"

	"github.com/nwolber/xCUTEr/job"
)

type Timing struct {
	start time.Time
	nodes map[string]*timingNode

	JobRuntime time.Duration
	Hosts      map[*job.Host]*timingNode
}

// NewTiming generates a new Timing from the given config.
func NewTiming(c *job.Config) (*Timing, error) {
	builder := newTimingBuilder()
	_, err := job.VisitConfig(builder, c)
	if err != nil {
		return nil, err
	}

	return &Timing{
		nodes: builder.(*NamingBuilder).NamedConfigBuilder.(*timingBuilder).nodes,
		Hosts: builder.(*NamingBuilder).NamedConfigBuilder.(*timingBuilder).hosts,
	}, nil
}

// ApplyChan progressivly applies the job events sent
// through the channel to the Timing.
func (v *Timing) ApplyChan(events <-chan Event) {
	for event := range events {
		v.Apply(event)
	}
}

// ApplyStore applies all events to the Timing.
func (v *Timing) ApplyStore(events []Event) {
	for _, event := range events {
		v.Apply(event)
	}
}

// Apply applies an event to the Timing.
func (v *Timing) Apply(event Event) {
	if (v.start == time.Time{}) && event.Type == EventStart {
		v.start = event.Timestamp
	}

	node, ok := v.nodes[event.Name]
	if !ok {
		return
	}

	switch event.Type {
	case EventStart:
		node.start = event.Timestamp
	case EventEnd:
		fallthrough
	case EventFailed:
		node.Runtime = event.Timestamp.Sub(node.start)

		if (v.start != time.Time{}) {
			v.JobRuntime = event.Timestamp.Sub(v.start)
		}
	}
}

type noopGroup struct{}

func (g *noopGroup) Append(children ...interface{}) {}
func (g *noopGroup) Wrap() interface{}              { return nil }

type timingNode struct {
	start   time.Time
	Runtime time.Duration
}

type timingBuilder struct {
	nodes map[string]*timingNode
	hosts map[*job.Host]*timingNode
}

func (t *timingBuilder) storeNode(nodeName string, host *job.Host) {
	node := &timingNode{}
	t.nodes[nodeName] = node
	t.hosts[host] = node
}

func newTimingBuilder() job.ConfigBuilder {
	return &NamingBuilder{
		NamedConfigBuilder: &timingBuilder{
			nodes: make(map[string]*timingNode),
			hosts: make(map[*job.Host]*timingNode),
		},
	}
}

func (t *timingBuilder) Sequential(nodeName string) job.Group {
	return &noopGroup{}
}

func (t *timingBuilder) Parallel(nodeName string) job.Group {
	return &noopGroup{}
}

func (t *timingBuilder) Job(nodeName string, name string) job.Group {
	return &noopGroup{}
}

func (t *timingBuilder) Output(nodeName string, o *job.Output) interface{} {
	return nil
}

func (t *timingBuilder) JobLogger(nodeName string, jobName string) interface{} {
	return nil
}

func (t *timingBuilder) HostLogger(nodeName string, jobName string, h *job.Host) interface{} {
	return nil
}

func (t *timingBuilder) Timeout(nodeName string, timeout time.Duration) interface{} {
	return nil
}

func (t *timingBuilder) SCP(nodeName string, scp *job.ScpData) interface{} {
	return nil
}

func (t *timingBuilder) Hosts(nodeName string) job.Group {
	return &noopGroup{}
}

func (t *timingBuilder) Host(nodeName string, c *job.Config, h *job.Host) job.Group {
	t.storeNode(nodeName, h)
	return &noopGroup{}
}

func (t *timingBuilder) ErrorSafeguard(nodeName string, child interface{}) interface{} {
	return nil
}

func (t *timingBuilder) ContextBounds(nodeName string, child interface{}) interface{} {
	return nil
}

func (t *timingBuilder) Retry(nodeName string, child interface{}, retries uint) interface{} {
	return nil
}

func (t *timingBuilder) Templating(nodeName string, c *job.Config, h *job.Host) interface{} {
	return nil
}

func (t *timingBuilder) SSHClient(nodeName string, host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return nil
}

func (t *timingBuilder) Forwarding(nodeName string, f *job.Forwarding) interface{} {
	return nil
}

func (t *timingBuilder) Tunnel(nodeName string, f *job.Forwarding) interface{} {
	return nil
}

func (t *timingBuilder) Commands(nodeName string, cmd *job.Command) job.Group {
	return &noopGroup{}
}

func (t *timingBuilder) Command(nodeName string, cmd *job.Command) interface{} {
	return nil
}

func (t *timingBuilder) LocalCommand(nodeName string, cmd *job.Command) interface{} {
	return nil
}

func (t *timingBuilder) Stdout(nodeName string, o *job.Output) interface{} {
	return nil
}

func (t *timingBuilder) Stderr(nodeName string, o *job.Output) interface{} {
	return nil
}
