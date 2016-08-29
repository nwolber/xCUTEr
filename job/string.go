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

type stringVisitor struct {
	full, raw             bool
	maxHosts, maxCommands int
}

type vars struct {
	tt *templatingEngine
}

type stringer interface {
	String(v *vars) string
}

type simple string

func (s simple) String(v *vars) string {
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
	stringers []stringer
	max       int
	raw       bool
}

func (s *multiple) Append(children ...interface{}) {
	for _, cc := range children {
		if cc == nil {
			continue
		}

		f, ok := cc.(stringer)
		if !ok {
			log.Panicf("not a Stringer %T", cc)
		}

		s.stringers = append(s.stringers, f)
	}
}

func (s *multiple) Wrap() interface{} {
	return s
}

func (s *multiple) String(v *vars) string {
	str := s.typ
	l := len(s.stringers)

	if l <= 0 {
		return str
	}

	if s.raw {
		v = &vars{}
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

func (*stringVisitor) Sequential() group {
	return &multiple{
		typ: "Sequential",
	}
}

func (*stringVisitor) Parallel() group {
	return &multiple{
		typ: "Parallel",
	}
}

func (s *stringVisitor) Job(name string) group {
	return &multiple{
		typ: name,
		raw: s.raw,
	}
}

func (s *stringVisitor) Output(file string) interface{} {
	if file == "" {
		return nil
	}

	return simple("Output: " + file)
}

func (s *stringVisitor) JobLogger(jobName string) interface{} {
	if s.full {
		return simple("Create job logger")
	}
	return nil
}

func (s *stringVisitor) HostLogger(jobName string, h *host) interface{} {
	if s.full {
		return simple("Create host logger")
	}
	return nil
}

func (*stringVisitor) Timeout(timeout time.Duration) interface{} {
	return simple(fmt.Sprint("Timeout: ", timeout))
}

func (*stringVisitor) SCP(scp *scpData) interface{} {
	return simple(fmt.Sprintf("SCP listen on %s:%d", scp.Addr, scp.Port))
}

func (s *stringVisitor) Hosts() group {
	return &multiple{
		typ: "Target hosts",
		max: s.maxHosts,
	}
}

type partHost struct {
	*multiple
	h *host
}

func (p *partHost) Append(children ...interface{}) {
	p.multiple.Append(children...)
}

func (p *partHost) Wrap() interface{} {
	return p
}

func (p *partHost) String(v *vars) string {
	vr := &vars{}

	if v != nil && v.tt != nil {
		vr.tt = &templatingEngine{
			Config: v.tt.Config,
			Host:   p.h,
			now:    v.tt.now,
		}
	}

	return p.multiple.String(vr)
}

func (s *stringVisitor) Host(c *Config, h *host) group {
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

func (s *stringVisitor) ErrorSafeguard(child interface{}) interface{} {
	str, ok := child.(stringer)
	if !ok {
		log.Panicf("not a Stringer %T", child)
	}

	if s.full {
		return &multiple{
			typ: "Error safeguard",
			stringers: []stringer{
				str,
			},
		}
	}
	return child
}

func (s *stringVisitor) ContextBounds(child interface{}) interface{} {
	str, ok := child.(stringer)
	if !ok {
		log.Panicf("not a Stringer %T", child)
	}

	if s.full {
		return &multiple{
			typ: "Context Bounds",
			stringers: []stringer{
				str,
			},
		}
	}
	return child
}

func (s *stringVisitor) Retry(child interface{}, retries uint) interface{} {
	str, ok := child.(stringer)
	if !ok {
		log.Panicf("not a Stringer %T", child)
	}

	return &multiple{
		typ: fmt.Sprintf("Retry up to %d times", retries),
		stringers: []stringer{
			str,
		},
	}
}

func (s *stringVisitor) Templating(c *Config, h *host) interface{} {
	if s.full {
		return simple("Create templating engine")
	}
	return nil
}

func (*stringVisitor) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return simple(fmt.Sprintf("Open SSH connection to %s@%s", user, host))
}

func (*stringVisitor) Forwarding(f *forwarding) interface{} {
	return simple(fmt.Sprintf("Forward %s:%d to %s:%d", f.RemoteHost, f.RemotePort, f.LocalHost, f.LocalPort))
}

func (*stringVisitor) Tunnel(f *forwarding) interface{} {
	return simple(fmt.Sprintf("Tunnel %s:%d to %s:%d", f.LocalHost, f.LocalPort, f.RemoteHost, f.RemotePort))
}

func (s *stringVisitor) Commands(cmd *command) group {
	return &multiple{
		typ: "Command",
		max: s.maxCommands,
	}
}

func (s *stringVisitor) Command(cmd *command) interface{} {
	var str string
	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q", cmd.Command)
	} else {
		str = "!!! ERROR !!!"
	}

	return simple(str)
}

func (s *stringVisitor) LocalCommand(cmd *command) interface{} {
	var str string
	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q locally", cmd.Command)
	} else {
		str = "!!! ERROR !!!"
	}

	return simple(str)
}

func (s *stringVisitor) Stdout(file string) interface{} {
	if file == "null" {
		return simple("Discard any output from STDOUT")
	}

	return simple("Redirect STDOUT to " + file)
}

func (s *stringVisitor) Stderr(file string) interface{} {
	if file == "null" {
		return simple("Discard any output from STDERR")
	}

	return simple("Redirect STDERR to " + file)
}
