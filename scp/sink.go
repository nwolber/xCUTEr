package scp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

		switch m.typ {
		case msgTypeC:
			err = s.processCMessage(m)
		case msgTypeD:
			err = s.processDMessage(m)
		case msgTypeE:
			err = s.processEMessage(m)
		default:
			return fmt.Errorf("parser: expected type: C, D or E, found: %q", m.typ)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func parseSCPMessage(input []byte, recursive bool) (scpMessage, error) {
	var m scpMessage
	l, items := lex(input)
	l.recursive = recursive

	item, ok := <-items
	if !ok {
		return m, nil
	}

	if item.itemType != itemTyp {
		return m, fmt.Errorf("parser: expected: %q, found: %q", itemTyp, item.itemType)
	}

	m.typ = item.val

	for _, b := range binders(&m) {
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

func (s *scpImp) processCMessage(msg scpMessage) error {
	s.l.Printf("received C-message %s", msg)

	path := filepath.Join(filePath(s.dir, msg.fileName), msg.fileName)
	f, err := s.openFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, msg.fileMode)
	if err != nil {
		return err
	}
	defer f.Close()

	ack(s.out)

	// TODO: read from underlying reader without buffering
	fileReader := io.LimitReader(s.in, int64(msg.length))
	io.Copy(f, fileReader)

	r, err := s.in.ReadByte()
	if err != nil {
		return err
	}

	if r != 0 {
		return fmt.Errorf("parser: expected: %q, found: %q", '\x00', r)
	}
	ack(s.out)

	return nil
}

func (s *scpImp) processDMessage(msg scpMessage) error {
	s.l.Printf("received D-message %s", msg)

	if msg.length != 0 {
		return errInvalidLength
	}

	s.dir = filepath.Join(s.dir, msg.fileName)

	s.mkdir(s.dir, msg.fileMode)

	ack(s.out)

	return nil
}

func (s *scpImp) processEMessage(msg scpMessage) error {
	s.l.Printf("received E-message %s", msg)

	s.dir, _ = filepath.Split(s.dir)
	s.dir = filepath.Clean(s.dir)

	ack(s.out)

	return nil
}

type binder struct {
	typ  itemType
	bind func(string) error
}

func binders(m *scpMessage) []binder {
	if m.typ == msgTypeE {
		return []binder{
			binder{itemEnd, func(val string) error { return nil }},
		}
	}

	return []binder{
		binder{itemPerm, func(val string) (err error) {
			mode, err := strconv.ParseUint(val, 8, 32)
			m.fileMode = os.FileMode(mode)
			return
		}},
		binder{itemSize, func(val string) (err error) {
			m.length, err = strconv.ParseUint(val, 10, 64)
			return
		}},
		binder{itemName, func(val string) error {
			m.fileName = val
			return nil
		}},
		binder{itemEnd, func(val string) error { return nil }},
	}
}
