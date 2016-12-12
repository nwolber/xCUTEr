// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"log"
	"strings"
	"time"
)

type StringBuilder struct {
	Full, Raw             bool
	MaxHosts, MaxCommands int
}

type Vars struct {
	tt *TemplatingEngine
}

type Stringer interface {
	String(v *Vars) string
}

type simple string

func (s simple) String(v *Vars) string {
	str := string(s)
	if v != nil && v.tt != nil {
		newStr, err := v.tt.Interpolate(str)
		if err == nil {
			str = newStr
		} else {
			log.Println(err)
		}
	}

	return str
}

type multiple struct {
	typ       string
	stringers []Stringer
	max       int
	raw       bool
}

func (s *multiple) Append(children ...interface{}) {
	for _, cc := range children {
		if cc == nil {
			continue
		}

		f, ok := cc.(Stringer)
		if !ok {
			log.Panicf("not a Stringer %T", cc)
		}

		s.stringers = append(s.stringers, f)
	}
}

func (s *multiple) Wrap() interface{} {
	return s
}

func (s *multiple) String(v *Vars) string {
	str := s.typ
	l := len(s.stringers)

	if l <= 0 {
		return str
	}

	if s.raw {
		v = &Vars{}
	}

	str += "\n"

	if s.max > 0 && l > s.max {
		for i := 0; i < s.max; i++ {
			sub := s.stringers[i].String(v)
			sub = strings.Replace(sub, "\n", "\n│  ", -1)

			str += "├─ " + sub + "\n"
		}
		str += fmt.Sprintf("└─ and %d more ...", l-s.max)
	} else {
		for i := 0; i < l-1; i++ {
			sub := s.stringers[i].String(v)
			sub = strings.Replace(sub, "\n", "\n│  ", -1)

			str += "├─ " + sub + "\n"
		}

		sub := s.stringers[l-1].String(v)
		sub = strings.Replace(sub, "\n", "\n   ", -1)
		str += "└─ " + sub
	}

	return str
}

func (*StringBuilder) Sequential() Group {
	return &multiple{
		typ: "Sequential",
	}
}

func (*StringBuilder) Parallel() Group {
	return &multiple{
		typ: "Parallel",
	}
}

func (s *StringBuilder) Job(name string) Group {
	return &multiple{
		typ: name,
		raw: s.Raw,
	}
}

func (s *StringBuilder) Output(o *Output) interface{} {
	if o == nil {
		return nil
	}

	return simple(fmt.Sprintf("Output: %s", o))
}

func (s *StringBuilder) JobLogger(jobName string) interface{} {
	if s.Full {
		return simple("Create job logger")
	}
	return nil
}

func (s *StringBuilder) HostLogger(jobName string, h *Host) interface{} {
	if s.Full {
		return simple("Create host logger")
	}
	return nil
}

func (*StringBuilder) Timeout(timeout time.Duration) interface{} {
	return simple(fmt.Sprint("Timeout: ", timeout))
}

func (*StringBuilder) SCP(scp *ScpData) interface{} {
	return simple(fmt.Sprintf("SCP listen on %s:%d", scp.Addr, scp.Port))
}

func (s *StringBuilder) Hosts() Group {
	return &multiple{
		typ: "Target hosts",
		max: s.MaxHosts,
	}
}

type partHost struct {
	*multiple
	h *Host
}

func (p *partHost) Append(children ...interface{}) {
	p.multiple.Append(children...)
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

	return p.multiple.String(vr)
}

func (s *StringBuilder) Host(c *Config, h *Host) Group {
	var name string

	if h.Name != "" {
		name = h.Name
	} else {
		name = h.Addr
	}

	p := &partHost{
		multiple: &multiple{
			typ: "Host " + name,
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
		return &multiple{
			typ: "Error safeguard",
			stringers: []Stringer{
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
		return &multiple{
			typ: "Context Bounds",
			stringers: []Stringer{
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

	return &multiple{
		typ: fmt.Sprintf("Retry up to %d times", retries),
		stringers: []Stringer{
			str,
		},
	}
}

func (s *StringBuilder) Templating(c *Config, h *Host) interface{} {
	if s.Full {
		return simple("Create templating engine")
	}
	return nil
}

func (*StringBuilder) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return simple(fmt.Sprintf("Open SSH connection to %s@%s", user, host))
}

func (*StringBuilder) Forwarding(f *Forwarding) interface{} {
	return simple(fmt.Sprintf("Forward %s:%d to %s:%d", f.RemoteHost, f.RemotePort, f.LocalHost, f.LocalPort))
}

func (*StringBuilder) Tunnel(f *Forwarding) interface{} {
	return simple(fmt.Sprintf("Tunnel %s:%d to %s:%d", f.LocalHost, f.LocalPort, f.RemoteHost, f.RemotePort))
}

func (s *StringBuilder) Commands(cmd *Command) Group {
	return &multiple{
		typ: "Command",
		max: s.MaxCommands,
	}
}

func (s *StringBuilder) Command(cmd *Command) interface{} {
	var str string
	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q", cmd.Command)
	} else {
		str = "!!! ERROR !!!"
	}

	return simple(str)
}

func (s *StringBuilder) LocalCommand(cmd *Command) interface{} {
	var str string
	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q locally", cmd.Command)
	} else {
		str = "!!! ERROR !!!"
	}

	return simple(str)
}

func (s *StringBuilder) Stdout(o *Output) interface{} {
	if o.File == "null" {
		return simple("Discard any output from STDOUT")
	}

	return simple(fmt.Sprintf("Redirect STDOUT to %s", o))
}

func (s *StringBuilder) Stderr(o *Output) interface{} {
	if o.File == "null" {
		return simple("Discard any output from STDERR")
	}

	return simple(fmt.Sprintf("Redirect STDERR to %s", o))
}
