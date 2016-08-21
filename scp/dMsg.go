package scp

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type scpDMessage struct {
	mode   os.FileMode
	length uint64
	name   string
}

func (m scpDMessage) String() string {
	return fmt.Sprintf("D%04o %d %s\n", uint32(m.mode), m.length, m.name)
}

func (m *scpDMessage) binders() []binder {
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

func (msg scpDMessage) process(s *scpImp) error {
	s.l.Printf("received D-message %s", msg)

	if msg.length != 0 {
		return errInvalidLength
	}

	s.dir = filepath.Join(s.dir, msg.name)
	if err := s.mkdir(s.dir, msg.mode); err != nil {
		return err
	}

	ack(s.out)

	return nil
}
