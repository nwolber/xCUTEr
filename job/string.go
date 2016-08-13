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
	full bool
	tt   *templatingEngine
}

type simple string

func (s simple) String() string {
	return string(s)
}

type multiple struct {
	typ       string
	stringers []fmt.Stringer
}

func (s *multiple) Append(children ...interface{}) {
	for _, cc := range children {
		if cc == nil {
			continue
		}

		f, ok := cc.(fmt.Stringer)
		if !ok {
			log.Panicf("not a fmt.Stringer %T", cc)
		}

		s.stringers = append(s.stringers, f)
	}
}

func (s *multiple) Wrap() interface{} {
	return s
}

func (s *multiple) String() string {
	str := s.typ
	l := len(s.stringers)

	if l <= 0 {
		return str
	}

	str += "\n"

	for i := 0; i < len(s.stringers)-1; i++ {
		sub := s.stringers[i].String()
		sub = strings.Replace(sub, "\n", "\n│  ", -1)

		str += "├─ " + sub + "\n"
	}

	sub := s.stringers[l-1].String()
	sub = strings.Replace(sub, "\n", "\n   ", -1)
	str += "└─ " + sub

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

func (*stringVisitor) Job(name string) group {
	return &multiple{
		typ: name,
	}
}

func (s *stringVisitor) Output(file string) interface{} {
	if file == "" {
		return nil
	}

	if s.tt != nil {
		newFile, err := s.tt.Interpolate(file)
		if err == nil {
			file = newFile
		}
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

func (*stringVisitor) SCP(scp *scp) interface{} {
	return simple("SCP listen on " + scp.Addr)
}

func (*stringVisitor) Host(c *config, h *host) group {
	return &multiple{
		typ: "Host " + h.Addr,
	}
}

func (s *stringVisitor) ErrorSafeguard(child interface{}) interface{} {
	stringer, ok := child.(fmt.Stringer)
	if !ok {
		log.Panicf("not a fmt.Stringer %T", child)
	}

	if s.full {
		return &multiple{
			typ: "Error safeguard",
			stringers: []fmt.Stringer{
				stringer,
			},
		}
	}
	return child
}

func (s *stringVisitor) Templating(c *config, h *host) interface{} {
	if s.full {
		return simple("Create templating engine")
	}
	return nil
}

func (*stringVisitor) SSHClient(host, user string) interface{} {
	return simple(fmt.Sprintf("Open SSH connection to %s@%s", user, host))
}

func (*stringVisitor) Forwarding(f *forwarding) interface{} {
	return simple(fmt.Sprintf("Forward %s:%d to %s:%d", f.RemoteHost, f.RemotePort, f.LocalHost, f.LocalPort))
}

func (s *stringVisitor) Commands(cmd *command) group {
	return &multiple{
		// typ: s.Command(cmd).(fmt.Stringer).String(),
		typ: "Command",
	}
}

func (*stringVisitor) Command(cmd *command) interface{} {
	var str string
	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q", cmd.Command)
	} else {
		str = "!!! ERROR !!!"
	}

	return simple(str)
}

func (*stringVisitor) Stdout(file string) interface{} {
	return simple("Redirect STDOUT to " + file)
}

func (*stringVisitor) Stderr(file string) interface{} {
	return simple("Redirect STDERR to " + file)
}
