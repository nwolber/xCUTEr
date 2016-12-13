// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package telemetry

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nwolber/xCUTEr/job"
	uuid "github.com/satori/go.uuid"
)

var (
	telemetryNamespaceUUID = uuid.FromStringOrNil("bf158ec0-41e8-4d19-8974-faab217ae760")
)

type counter struct {
	i uint64
}

func (c *counter) nextName() string {
	return uuid.NewV5(telemetryNamespaceUUID, fmt.Sprint(atomic.AddUint64(&c.i, uint64(1)))).String()
}

// NamedConfigBuilder is an interface for ConfigBuilders,
// that assign names to the generated nodes.
type NamedConfigBuilder interface {
	Sequential(nodeName string) job.Group
	Parallel(nodeName string) job.Group
	Job(nodeName string, name string) job.Group
	Output(nodeName string, o *job.Output) interface{}
	JobLogger(nodeName string, jobName string) interface{}
	HostLogger(nodeName string, jobName string, h *job.Host) interface{}
	Timeout(nodeName string, timeout time.Duration) interface{}
	SCP(nodeName string, scp *job.ScpData) interface{}
	Hosts(nodeName string) job.Group
	Host(nodeName string, c *job.Config, h *job.Host) job.Group
	ErrorSafeguard(nodeName string, child interface{}) interface{}
	ContextBounds(nodeName string, child interface{}) interface{}
	Retry(nodeName string, child interface{}, retries uint) interface{}
	Templating(nodeName string, c *job.Config, h *job.Host) interface{}
	SSHClient(nodeName string, host, user, keyFile, password string, keyboardInteractive map[string]string) interface{}
	Forwarding(nodeName string, f *job.Forwarding) interface{}
	Tunnel(nodeName string, f *job.Forwarding) interface{}
	Commands(nodeName string, cmd *job.Command) job.Group
	Command(nodeName string, cmd *job.Command) interface{}
	LocalCommand(nodeName string, cmd *job.Command) interface{}
	Stdout(nodeName string, o *job.Output) interface{}
	Stderr(nodeName string, o *job.Output) interface{}
}

// NamingBuilder is a ConfigBuilder that assigns each node a unique name.
// The name of the current node is passed to the child ConfigBuilder.
type NamingBuilder struct {
	NamedConfigBuilder
	Counter counter
}

func (t *NamingBuilder) nextName() string {
	name := t.Counter.nextName()
	// fmt.Println(name)
	return name
}

func (t *NamingBuilder) Sequential() job.Group {
	return t.NamedConfigBuilder.Sequential("Sequential" + t.nextName())
}

func (t *NamingBuilder) Parallel() job.Group {
	return t.NamedConfigBuilder.Parallel("Parallel" + t.nextName())
}

func (t *NamingBuilder) Job(name string) job.Group {
	return t.NamedConfigBuilder.Job("Job"+t.nextName(), name)
}

func (t *NamingBuilder) Output(o *job.Output) interface{} {
	return t.NamedConfigBuilder.Output("Output"+t.nextName(), o)
}

func (t *NamingBuilder) JobLogger(jobName string) interface{} {
	return t.NamedConfigBuilder.JobLogger("Sequential"+t.nextName(), jobName)
}

func (t *NamingBuilder) HostLogger(jobName string, h *job.Host) interface{} {
	return t.NamedConfigBuilder.HostLogger("HostLogger"+t.nextName(), jobName, h)
}

func (t *NamingBuilder) Timeout(timeout time.Duration) interface{} {
	return t.NamedConfigBuilder.Timeout("Timeout"+t.nextName(), timeout)
}

func (t *NamingBuilder) SCP(scp *job.ScpData) interface{} {
	return t.NamedConfigBuilder.SCP("SCP"+t.nextName(), scp)
}

func (t *NamingBuilder) Hosts() job.Group {
	return t.NamedConfigBuilder.Hosts("Hosts" + t.nextName())
}

func (t *NamingBuilder) Host(c *job.Config, h *job.Host) job.Group {
	return t.NamedConfigBuilder.Host("Host"+t.nextName(), c, h)
}

func (t *NamingBuilder) ErrorSafeguard(child interface{}) interface{} {
	return t.NamedConfigBuilder.ErrorSafeguard("ErrorSafeguard"+t.nextName(), child)
}

func (t *NamingBuilder) ContextBounds(child interface{}) interface{} {
	return t.NamedConfigBuilder.ContextBounds("ContextBounds"+t.nextName(), child)
}

func (t *NamingBuilder) Retry(child interface{}, retries uint) interface{} {
	return t.NamedConfigBuilder.Retry("Retry"+t.nextName(), child, retries)
}

func (t *NamingBuilder) Templating(c *job.Config, h *job.Host) interface{} {
	return t.NamedConfigBuilder.Templating("Templating"+t.nextName(), c, h)
}

func (t *NamingBuilder) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return t.NamedConfigBuilder.SSHClient("SSHClient"+t.nextName(), host, user, keyFile, password, keyboardInteractive)
}

func (t *NamingBuilder) Forwarding(f *job.Forwarding) interface{} {
	return t.NamedConfigBuilder.Forwarding("Forwarding"+t.nextName(), f)
}

func (t *NamingBuilder) Tunnel(f *job.Forwarding) interface{} {
	return t.NamedConfigBuilder.Tunnel("Tunnel"+t.nextName(), f)
}

func (t *NamingBuilder) Commands(cmd *job.Command) job.Group {
	return t.NamedConfigBuilder.Commands("Commands"+t.nextName(), cmd)
}

func (t *NamingBuilder) Command(cmd *job.Command) interface{} {
	return t.NamedConfigBuilder.Command("Command"+t.nextName(), cmd)
}

func (t *NamingBuilder) LocalCommand(cmd *job.Command) interface{} {
	return t.NamedConfigBuilder.LocalCommand("LocalCommand"+t.nextName(), cmd)
}

func (t *NamingBuilder) Stdout(o *job.Output) interface{} {
	return t.NamedConfigBuilder.Stdout("Stdout"+t.nextName(), o)
}

func (t *NamingBuilder) Stderr(o *job.Output) interface{} {
	return t.NamedConfigBuilder.Stderr("Stderr"+t.nextName(), o)
}
