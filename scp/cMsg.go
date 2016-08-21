package scp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

type scpCMessage struct {
	mode   os.FileMode
	length uint64
	name   string
}

func (m scpCMessage) String() string {
	return fmt.Sprintf("C%04o %d %s\n", uint32(m.mode), m.length, m.name)
}

func (m *scpCMessage) binders() []binder {
	return []binder{
		binder{itemPerm, func(val string) (err error) {
			mode, err := strconv.ParseUint(val, 8, 32)
			m.mode = os.FileMode(mode)
			return
		}},
		binder{itemSize, func(val string) (err error) {
			m.length, err = strconv.ParseUint(val, 10, 64)
			return
		}},
		binder{itemName, func(val string) error {
			m.name = val
			return nil
		}},
		binder{itemEnd, func(val string) error { return nil }},
	}
}

func (msg *scpCMessage) process(s *scpImp) error {
	s.l.Printf("received C-message %s", msg)

	path := filepath.Join(filePath(s.dir, msg.name), msg.name)
	f, err := s.openFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, msg.mode)
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
