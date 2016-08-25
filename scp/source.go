// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func (s *scpImp) runSource() error {
	if err := getAck(s.in); err != nil {
		return err
	}

	if s.recursive {
		return s.sourceRecurse()
	}
	return s.sourceNoRecurse()
}

func (s *scpImp) sourceRecurse() error {
	f, err := s.stat(s.dir)
	if err != nil {
		return err
	}

	if f.IsDir() {
		return s.sendDirectory(f)
	}
	return s.sendFile(f)
}

func (s *scpImp) sourceNoRecurse() error {
	f, err := s.stat(s.dir)
	if err != nil {
		return err
	}

	if f.IsDir() {
		return fmt.Errorf("%s is not a regular file", s.name)
	}

	return s.sendFile(f)
}

func (s *scpImp) sendDirectory(f FileInfo) error {
	if s.times {
		if err := s.sendFileTimes(f); err != nil {
			return err
		}
	}

	msg := &scpDMessage{
		name: f.Name(),
		mode: f.Mode() & os.ModePerm,
	}
	s.l.Printf("sending D message: %s", msg)

	if _, err := fmt.Fprint(s.out, msg); err != nil {
		return err
	}

	if err := getAck(s.in); err != nil {
		return err
	}

	files, err := s.readDir(s.dir)
	if err != nil {
		return err
	}

	for _, fi := range files {
		if fi.IsDir() {
			s.dir = filepath.Join(s.dir, fi.Name())
			if err := s.sendDirectory(fi); err != nil {
				return err
			}
			s.dir, _ = filepath.Split(s.dir)
			s.dir = filepath.Clean(s.dir)
		} else {
			if err := s.sendFile(fi); err != nil {
				return err
			}
		}
	}

	m := &scpEMessage{}
	s.l.Printf("sending E message: %s", m)
	if _, err := fmt.Fprint(s.out, m); err != nil {
		return err
	}

	if err := getAck(s.in); err != nil {
		return err
	}

	return nil
}

func (s *scpImp) sendFileTimes(f FileInfo) error {
	msg := &scpTMessage{
		aTime: f.AccessTime(),
		mTime: f.ModTime(),
	}
	s.l.Printf("sending T message: %s", msg)
	if _, err := fmt.Fprint(s.out, msg); err != nil {
		return err
	}

	if err := getAck(s.in); err != nil {
		return err
	}
	return nil
}

func (s *scpImp) sendFile(f FileInfo) error {
	if s.times {
		if err := s.sendFileTimes(f); err != nil {
			return err
		}
	}

	msg := &scpCMessage{
		name:   f.Name(),
		mode:   f.Mode() & os.ModePerm,
		length: uint64(f.Size()),
	}
	s.l.Printf("sending C message: %s", msg)

	if _, err := fmt.Fprint(s.out, msg); err != nil {
		return err
	}

	if err := getAck(s.in); err != nil {
		return err
	}

	file, err := s.openFile(path(s.dir, f.Name()), os.O_RDONLY, 0)
	if err != nil {
		return err
	}

	n, err := io.Copy(s.out, file)
	if err != nil {
		return err
	}
	s.l.Printf("%d bytes transferred", n)

	if _, err = fmt.Fprint(s.out, "\x00"); err != nil {
		return err
	}

	if err = getAck(s.in); err != nil {
		return err
	}

	return nil
}

func path(maybeDir, file string) string {
	if filepath.Base(maybeDir) == file {
		return maybeDir
	}

	return filepath.Join(maybeDir, file)
}

func getAck(in *bufio.Reader) error {
	b, err := in.ReadByte()
	if err != nil {
		return err
	}

	if b == 1 || b == 2 {
		str, err := in.ReadString('\n')
		if err != nil {
			return err
		}
		return fmt.Errorf("received error: %s", str)
	} else if b != 0 {
		return fmt.Errorf("parser: expected: \x00, got: %d", b)
	}

	return nil
}
