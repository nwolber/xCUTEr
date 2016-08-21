// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"fmt"
	"io"
)

func (s *scpImp) runSink() error {
	ack(s.out)
	p := parser{
		s: s,
	}

	for state := p.parserStart(s.recursive, s.times); state != nil; {
		input, err := s.in.ReadBytes('\n')
		if err == io.EOF {
			s.l.Println("eof")
			return nil
		} else if err != nil {
			return err
		}

		m, err := parseSCPMessage(input)
		if err != nil {
			return err
		}

		state = state(&p, m)
	}

	return p.err
}

type parserStateFn func(p *parser, m scpMessage) parserStateFn

type parser struct {
	s     *scpImp
	depth int
	err   error
}

func (p *parser) error(err error) {
	p.err = err
}

func (p *parser) parserStart(recursive, times bool) parserStateFn {
	if recursive && times {
		return stateRecurseTimesCDT
	} else if recursive && !times {
		return stateRecurseNoTimesCD
	} else if !recursive && times {
		return stateNoRecurseTimesCT
	} else if !recursive && !times {
		return stateNoRecurseNoTimesC
	}
	panic("should never be reached")
}

func stateNoRecurseTimesCT(p *parser, msg scpMessage) (next parserStateFn) {
	switch msg.(type) {
	case *scpCMessage:
		next = stateNoRecurseTimesCT
	case *scpTMessage:
		next = stateNoRecurseTimesC
	default:
		p.error(fmt.Errorf("unexpected message in state CT: %q", msg))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func stateNoRecurseTimesC(p *parser, msg scpMessage) (next parserStateFn) {
	switch m := msg.(type) {
	case *scpCMessage:
		next = stateNoRecurseTimesCT
	default:
		p.error(fmt.Errorf("unexpected message in state C: %q", m))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func stateNoRecurseNoTimesC(p *parser, msg scpMessage) (next parserStateFn) {
	switch m := msg.(type) {
	case *scpCMessage:
		next = stateNoRecurseNoTimesC
	default:
		p.error(fmt.Errorf("unexpected message in state C: %q", m))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func stateRecurseTimesCDT(p *parser, msg scpMessage) (next parserStateFn) {
	switch msg.(type) {
	case *scpCMessage:
		next = stateRecurseTimesCDT
	case *scpDMessage:
		next = stateRecurseTimesCDET
		p.depth++
	case *scpTMessage:
		next = stateRecurseTimesCD
	default:
		p.error(fmt.Errorf("unexpected message in state CDT: %q", msg))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func stateRecurseTimesCDET(p *parser, msg scpMessage) (next parserStateFn) {
	switch msg.(type) {
	case *scpCMessage:
		next = stateRecurseTimesCDET
	case *scpDMessage:
		next = stateRecurseTimesCDET
		p.depth++
	case *scpEMessage:
		p.depth--
		if p.depth == 0 {
			next = stateRecurseTimesCDT
		} else {
			next = stateRecurseTimesCDET
		}
	case *scpTMessage:
		next = stateRecurseTimesCD
	default:
		p.error(fmt.Errorf("unexpected message in state CDET: %q", msg))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func stateRecurseTimesCD(p *parser, msg scpMessage) (next parserStateFn) {
	switch msg.(type) {
	case *scpCMessage:
		next = stateRecurseTimesCDET
	case *scpDMessage:
		next = stateRecurseTimesCDET
		p.depth++
	default:
		p.error(fmt.Errorf("unexpected message in state CD: %q", msg))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func stateRecurseNoTimesCD(p *parser, msg scpMessage) (next parserStateFn) {
	switch msg.(type) {
	case *scpCMessage:
		next = stateRecurseNoTimesCD
	case *scpDMessage:
		next = stateRecurseNoTimesCDE
		p.depth++
	default:
		p.error(fmt.Errorf("unexpected message in state CD: %q", msg))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func stateRecurseNoTimesCDE(p *parser, msg scpMessage) (next parserStateFn) {
	switch msg.(type) {
	case *scpCMessage:
		next = stateRecurseNoTimesCDE
	case *scpDMessage:
		next = stateRecurseNoTimesCDE
		p.depth++
	case *scpEMessage:
		p.depth--
		if p.depth == 0 {
			next = stateRecurseNoTimesCD
		} else {
			next = stateRecurseNoTimesCDE
		}
	default:
		p.error(fmt.Errorf("unexpected message in state CD: %q", msg))
		return nil
	}
	if err := msg.process(p.s); err != nil {
		p.error(err)
		return nil
	}
	return
}

func parseSCPMessage(input []byte) (scpMessage, error) {
	_, items := lex(input)

	item, ok := <-items
	if !ok {
		return nil, io.EOF
	}

	if item.itemType != itemTyp {
		return nil, fmt.Errorf("parser: expected: %q, found: %q", itemTyp, item.itemType)
	}

	const (
		msgTypeC = "C"
		msgTypeD = "D"
		msgTypeE = "E"
		msgTypeT = "T"
	)

	var m scpMessage
	switch item.val {
	case msgTypeC:
		m = &scpCMessage{}
	case msgTypeD:
		m = &scpDMessage{}
	case msgTypeE:
		m = &scpEMessage{}
	case msgTypeT:
		m = &scpTMessage{}
	default:
		return nil, fmt.Errorf("parser: expected type: C, D, T or E, found: %q", item.val)
	}

	for _, b := range m.binders() {
		item, ok := <-items
		if !ok {
			return nil, io.EOF
		}

		if item.itemType == b.typ {
			b.bind(item.val)
		} else {
			return nil, fmt.Errorf("parser: expected: %q, found: %q at %d", b.typ, item.itemType, item.pos)
		}
	}
	return m, nil
}

type binder struct {
	typ  itemType
	bind func(string) error
}

func nopBind(s string) error {
	return nil
}
