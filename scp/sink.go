package scp

import (
	"fmt"
	"io"
)

func (s *scpImp) runSink() error {
	ack(s.out)

	for {
		input, err := s.in.ReadBytes('\n')
		if err == io.EOF {
			s.l.Println("eof")
			return nil
		} else if err != nil {
			return err
		}

		m, err := parseSCPMessage(input, s.recursive)
		if err != nil {
			return err
		}

		if err := m.process(s); err != nil {
			return err
		}
	}

	return nil
}

func parseSCPMessage(input []byte, recursive bool) (scpMessage, error) {
	l, items := lex(input)
	l.recursive = recursive

	item, ok := <-items
	if !ok {
		return nil, io.EOF
	}

	if item.itemType != itemTyp {
		return nil, fmt.Errorf("parser: expected: %q, found: %q", itemTyp, item.itemType)
	}

	var m scpMessage
	switch item.val {
	case msgTypeC:
		m = &scpCMessage{}
	case msgTypeD:
		m = &scpDMessage{}
	case msgTypeE:
		m = &scpEMessage{}
	default:
		return nil, fmt.Errorf("parser: expected type: C, D or E, found: %q", item.val)
	}

	for _, b := range m.binders() {
		item, ok := <-items
		if !ok {
			return m, io.EOF
		}

		if item.itemType == b.typ {
			b.bind(item.val)
		} else {
			return m, fmt.Errorf("parser: expected: %q, found: %q", b.typ, item.itemType)
		}
	}
	return m, nil
}

type binder struct {
	typ  itemType
	bind func(string) error
}
