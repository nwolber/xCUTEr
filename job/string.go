// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"log"
	"time"
)

type StringBuilder struct {
	Full, Raw             bool
	MaxHosts, MaxCommands int
}

func (*StringBuilder) Sequential() Group {
	return &SimpleBranch{
		Root: "Sequential",
	}
}

func (*StringBuilder) Parallel() Group {
	return &SimpleBranch{
		Root: "Parallel",
	}
}

func (s *StringBuilder) Job(name string) Group {
	return &SimpleBranch{
		Root: Leaf(name),
		raw:  s.Raw,
	}
}

func (s *StringBuilder) Output(o *Output) interface{} {
	if o == nil {
		return nil
	}

	return Leaf(fmt.Sprintf("Output: %s", o))
}

func (s *StringBuilder) JobLogger(jobName string) interface{} {
	if s.Full {
		return Leaf("Create job logger")
	}
	return nil
}

func (s *StringBuilder) HostLogger(jobName string, h *Host) interface{} {
	if s.Full {
		return Leaf("Create host logger")
	}
	return nil
}

func (*StringBuilder) Timeout(timeout time.Duration) interface{} {
	return Leaf(fmt.Sprint("Timeout: ", timeout))
}

func (*StringBuilder) SCP(scp *ScpData) interface{} {
	return Leaf(fmt.Sprintf("SCP listen on %s:%d", scp.Addr, scp.Port))
}

func (s *StringBuilder) Hosts() Group {
	return &SimpleBranch{
		Root: "Target hosts",
		max:  s.MaxHosts,
	}
}

type partHost struct {
	*SimpleBranch
	h *Host
}

func (p *partHost) Append(children ...interface{}) {
	p.SimpleBranch.Append(children...)
}

func (p *partHost) Wrap() interface{} {
	return p
}

func (p *partHost) String(v *Vars) string {
	vr := &Vars{}

	if v != nil && v.tt != nil {
		vr.tt = &TemplatingEngine{
			Config: v.tt.Config,
			Host:   p.h,
			now:    v.tt.now,
		}
	}

	return p.SimpleBranch.String(vr)
}

func (s *StringBuilder) Host(c *Config, h *Host) Group {
	var name string

	if h.Name != "" {
		name = h.Name
	} else {
		name = h.Addr
	}

	p := &partHost{
		SimpleBranch: &SimpleBranch{
			Root: Leaf("Host " + name),
		},
		h: h,
	}

	return p
}

func (s *StringBuilder) ErrorSafeguard(child interface{}) interface{} {
	str, ok := child.(Stringer)
	if !ok {
		log.Panicf("not a Stringer %T", child)
	}

	if s.Full {
		return &SimpleBranch{
			Root: "Error safeguard",
			Leafs: []Stringer{
				str,
			},
		}
	}
	return child
}

func (s *StringBuilder) ContextBounds(child interface{}) interface{} {
	str, ok := child.(Stringer)
	if !ok {
		log.Panicf("not a Stringer %T", child)
	}

	if s.Full {
		return &SimpleBranch{
			Root: "Context Bounds",
			Leafs: []Stringer{
				str,
			},
		}
	}
	return child
}

func (s *StringBuilder) Retry(child interface{}, retries uint) interface{} {
	str, ok := child.(Stringer)
	if !ok {
		log.Panicf("not a Stringer %T", child)
	}

	return &SimpleBranch{
		Root: Leaf(fmt.Sprintf("Retry up to %d times", retries)),
		Leafs: []Stringer{
			str,
		},
	}
}

func (s *StringBuilder) Templating(c *Config, h *Host) interface{} {
	if s.Full {
		return Leaf("Create templating engine")
	}
	return nil
}

func (*StringBuilder) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return Leaf(fmt.Sprintf("Open SSH connection to %s@%s", user, host))
}

func (*StringBuilder) Forwarding(f *Forwarding) interface{} {
	return Leaf(fmt.Sprintf("Forward %s:%d to %s:%d", f.RemoteHost, f.RemotePort, f.LocalHost, f.LocalPort))
}

func (*StringBuilder) Tunnel(f *Forwarding) interface{} {
	return Leaf(fmt.Sprintf("Tunnel %s:%d to %s:%d", f.LocalHost, f.LocalPort, f.RemoteHost, f.RemotePort))
}

func (s *StringBuilder) Commands(cmd *Command) Group {
	return &SimpleBranch{
		Root: "Command",
		max:  s.MaxCommands,
	}
}

func (s *StringBuilder) Command(cmd *Command) interface{} {
	var str string
	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q", cmd.Command)
	} else {
		str = "!!! ERROR !!!"
	}

	return Leaf(str)
}

func (s *StringBuilder) LocalCommand(cmd *Command) interface{} {
	var str string
	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q locally", cmd.Command)
	} else {
		str = "!!! ERROR !!!"
	}

	return Leaf(str)
}

func (s *StringBuilder) Stdout(o *Output) interface{} {
	if o.File == "null" {
		return Leaf("Discard any output from STDOUT")
	}

	return Leaf(fmt.Sprintf("Redirect STDOUT to %s", o))
}

func (s *StringBuilder) Stderr(o *Output) interface{} {
	if o.File == "null" {
		return Leaf("Discard any output from STDERR")
	}

	return Leaf(fmt.Sprintf("Redirect STDERR to %s", o))
}
