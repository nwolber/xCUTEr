// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
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
		{itemNumber, func(val string) (err error) {
			mode, err := strconv.ParseUint(val, 8, 32)
			m.mode = os.FileMode(mode)
			return
		}},
		{itemSpace, nopBind},
		{itemNumber, func(val string) (err error) {
			m.length, err = strconv.ParseUint(val, 10, 64)
			return
		}},
		{itemSpace, nopBind},
		{itemName, func(val string) error {
			m.name = val
			return nil
		}},
		{itemEnd, nopBind},
	}
}

func (m *scpCMessage) process(s *scpImp) error {
	s.l.Printf("received C-message %s", m)

	path := filepath.Join(filePath(s.dir, m.name), m.name)
	err := func() error {
		f, err := s.openFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, m.mode)
		if err != nil {
			return err
		}
		defer f.Close()

		s.ack(s.out)

		// TODO: read from underlying reader without buffering
		fileReader := io.LimitReader(s.in, int64(m.length))
		n, err := io.Copy(f, fileReader)
		s.l.Printf("transferred %d bytes", n)
		return err
	}()

	if err != nil {
		return err
	}

	if s.timeSet {
		s.chtimes(path, s.aTime, s.mTime)
		s.timeSet = false
	}

	r, err := s.in.ReadByte()
	if err != nil {
		return err
	}

	if r != 0 {
		return fmt.Errorf("parser: expected: %q, found: %q", '\x00', r)
	}
	s.ack(s.out)

	return nil
}

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
		{itemNumber, func(val string) (err error) {
			mode, err := strconv.ParseUint(val, 8, 32)
			m.mode = os.FileMode(mode)
			return
		}},
		{itemSpace, nopBind},
		{itemNumber, func(val string) (err error) {
			m.length, err = strconv.ParseUint(val, 10, 64)
			return
		}},
		{itemSpace, nopBind},
		{itemName, func(val string) error {
			m.name = val
			return nil
		}},
		{itemEnd, func(val string) error { return nil }},
	}
}

func (m scpDMessage) process(s *scpImp) error {
	s.l.Printf("received D-message %s", m)

	if m.length != 0 {
		return errInvalidLength
	}

	s.dir = filepath.Join(s.dir, m.name)
	if err := s.mkdir(s.dir, m.mode); err != nil {
		return err
	}

	if s.timeSet {
		s.chtimes(s.dir, s.aTime, s.mTime)
		s.timeSet = false
	}

	s.ack(s.out)

	return nil
}

type scpEMessage struct {
}

func (m scpEMessage) String() string {
	return "E\n"
}

func (m *scpEMessage) binders() []binder {
	return []binder{
		{itemEnd, func(val string) error { return nil }},
	}
}

func (m scpEMessage) process(s *scpImp) error {
	s.l.Printf("received E-message %s", m)

	s.dir, _ = filepath.Split(s.dir)
	s.dir = filepath.Clean(s.dir)

	s.ack(s.out)

	return nil
}

type scpTMessage struct {
	aTime, mTime time.Time
}

func (m scpTMessage) String() string {
	return fmt.Sprintf("T%d 0 %d 0\n", m.mTime.Unix(), m.aTime.Unix())
}

func (m *scpTMessage) binders() []binder {
	return []binder{
		// mTime
		{itemNumber, func(val string) (err error) {
			m.mTime, err = parseUnix(val)
			return
		}},
		{itemSpace, nopBind},
		// mTime nanoseconds, always 0
		{itemNumber, nopBind},
		{itemSpace, nopBind},
		// aTime
		{itemNumber, func(val string) (err error) {
			m.aTime, err = parseUnix(val)
			return
		}},
		{itemSpace, nopBind},
		// aTime nanoseconds, always 0
		{itemNumber, nopBind},
		{itemEnd, nopBind},
	}
}

func (m scpTMessage) process(s *scpImp) error {
	s.l.Printf("received T-message %s", m)

	s.aTime = m.aTime
	s.mTime = m.mTime
	s.timeSet = true

	s.ack(s.out)
	return nil
}

func parseUnix(val string) (t time.Time, err error) {
	unix, err := strconv.ParseInt(val, 10, 64)
	t = time.Unix(unix, 0)
	return
}
