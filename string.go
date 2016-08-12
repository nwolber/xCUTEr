package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nwolber/xCUTEr/flunc"

	"golang.org/x/net/context"
)

type stackElement struct {
	current []fmt.Stringer
}

type stringerStack []*stackElement

func (s stringerStack) Push(v *stackElement) stringerStack {
	return append(s, v)
}

func (s stringerStack) Pop() (stringerStack, *stackElement) {
	l := len(s)

	if l == 0 {
		return s, nil
	}

	return s[:l-1], s[l-1]
}

func (s stringerStack) Peek() *stackElement {
	l := len(s)

	if l == 0 {
		return nil
	}

	return s[l-1]
}

type stringBuilder struct {
	stack stringerStack
	// current []fmt.Stringer
}

func newStringBuilder() *stringBuilder {
	return &stringBuilder{
		stack: stringerStack{
			&stackElement{},
		},
	}
}

func (s *stringBuilder) String() string {
	if len(s.stack) > 1 {
		log.Println("warning: there is more than one stringer on the stack")
	}

	elem := s.stack.Peek()
	return elem.current[0].String()
}

type multiple struct {
	typ       string
	stringers []fmt.Stringer
}

func (s *multiple) String() string {
	str := s.typ + "\n"

	l := len(s.stringers)
	for i := 0; i < len(s.stringers)-1; i++ {
		sub := s.stringers[i].String()
		sub = strings.Replace(sub, "\n", "\n│  ", -1)

		str += "├─ " + sub + "\n"
	}
	if l <= 0 {
		log.Println("l is", l)
		log.Println("str is", str)
		return str
	}

	sub := s.stringers[l-1].String()
	sub = strings.Replace(sub, "\n", "\n   ", -1)
	str += "└─ " + sub

	return str
}

func (s *stringBuilder) Sequential(children ...flunc.Flunc) flunc.Flunc {
	var elem *stackElement
	s.stack, elem = s.stack.Pop()

	if len(elem.current) == 0 {
		log.Println("pling")
		return nil
	}

	str := &multiple{
		typ:       "Sequential",
		stringers: elem.current,
	}
	elem = s.stack.Peek()
	elem.current = append(elem.current, str)
	// s.Stringer = str

	return nil
}

func (s *stringBuilder) Parallel(children ...flunc.Flunc) flunc.Flunc {
	var elem *stackElement
	s.stack, elem = s.stack.Pop()

	if len(elem.current) == 0 {
		log.Println("plong")
		return nil
	}

	str := &multiple{
		typ:       "Parallel",
		stringers: elem.current,
	}
	elem = s.stack.Peek()
	// elem.current = append([]fmt.Stringer{str}, elem.current...)
	elem.current = append(elem.current, str)
	// s.Stringer = str

	return nil
}

type stringBuilderGroup struct {
	s        *stringBuilder
	children []fmt.Stringer
}

func (s *stringBuilderGroup) Append(children ...flunc.Flunc) {
}

func (s *stringBuilderGroup) Fluncs() []flunc.Flunc {
	return nil
}

func (s *stringBuilder) Group(children ...flunc.Flunc) group {
	s.stack = s.stack.Push(&stackElement{})
	return &stringBuilderGroup{s: s}
}

type partTimeout struct {
	timeout time.Duration
}

func (t *partTimeout) String() string {
	return fmt.Sprint("timeout:", t.timeout)
}

func (s *stringBuilder) Timeout(timeout time.Duration) {
	str := &partTimeout{timeout: timeout}

	elem := s.stack.Peek()
	elem.current = append(elem.current, str)
	// s.Stringer = str
}

type partSCP struct {
	addr string
}

func (p *partSCP) String() string {
	return fmt.Sprint("SCP listen on", p.addr)
}

func (s *stringBuilder) DoSCP(ctx context.Context, privateKey []byte, addr string) error {
	str := &partSCP{addr: addr}
	elem := s.stack.Peek()
	elem.current = append(elem.current, str)
	// s.Stringer = str
	return nil
}

func (s *stringBuilder) Host(c *config, h *host, children ...flunc.Flunc) flunc.Flunc {
	var elem *stackElement
	s.stack, elem = s.stack.Pop()

	str := &multiple{
		typ:       fmt.Sprint("Host ", h.Addr),
		stringers: elem.current,
	}
	elem = s.stack.Peek()
	elem.current = append(elem.current, str)
	// s.Stringer = str
	return nil
}

type partSSHClient struct {
	host, user string
}

func (p *partSSHClient) String() string {
	return fmt.Sprintf("Open SSH connection to %s@%s", p.user, p.host)
}

func (s *stringBuilder) PrepareSSHClient(host, user string) flunc.Flunc {
	str := &partSSHClient{
		host: host,
		user: user,
	}
	elem := s.stack.Peek()
	elem.current = append(elem.current, str)
	// s.Stringer = str
	return nil
}

type partForward struct {
	remoteAddr, localAddr string
}

func (p *partForward) String() string {
	return fmt.Sprintf("forward %s to %s", p.remoteAddr, p.localAddr)
}

func (s *stringBuilder) Forward(client *sshClient, ctx context.Context, remoteAddr, localAddr string) {
	str := &partForward{remoteAddr: remoteAddr, localAddr: localAddr}
	elem := s.stack.Peek()
	elem.current = append(elem.current, str)
	// s.Stringer = str
}

type partCommand struct {
	cmd *command
}

func (p *partCommand) String() string {
	var str string
	cmd := p.cmd

	if cmd.Flow != "" {
		str = strings.Title(cmd.Flow) + " "
	} else {
		str = "Command "
	}

	if cmd.Name != "" {
		str += cmd.Name + " "
	}

	str += "{"

	if cmd.Stdout != "" {
		str += fmt.Sprintf(" Stdout: %q", cmd.Stdout)
	}

	if cmd.Stderr != "" {
		str += fmt.Sprintf(" Stderr: %q", cmd.Stderr)
	}

	str += " }"

	if cmd.Command != "" {
		str = fmt.Sprintf("Execute %q", cmd.Command)
	}

	return str
}

func (s *stringBuilder) Command(cmd *command, children ...flunc.Flunc) flunc.Flunc {
	str := &partCommand{cmd: cmd}

	if cmd.Command != "" {
		elem := s.stack.Peek()
		elem.current = append(elem.current, str)
	} else if cmd.Commands != nil {
		var elem *stackElement
		s.stack, elem = s.stack.Pop()

		mult := &multiple{
			typ:       str.String(),
			stringers: elem.current,
		}
		elem = s.stack.Peek()
		elem.current = append(elem.current, mult)
		// s.Stringer = str
	}
	return nil
}

func (s *stringBuilder) Job(name string, children ...flunc.Flunc) flunc.Flunc {
	var elem *stackElement
	s.stack, elem = s.stack.Pop()

	str := &multiple{
		typ:       name,
		stringers: elem.current,
	}
	elem = s.stack.Peek()
	elem.current = append(elem.current, str)
	// s.Stringer = str
	return nil
}
