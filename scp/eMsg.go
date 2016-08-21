// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import "path/filepath"

type scpEMessage struct {
}

func (m scpEMessage) String() string {
	return "E\n"
}

func (m *scpEMessage) binders() []binder {
	return []binder{
		binder{itemEnd, func(val string) error { return nil }},
	}
}

func (m scpEMessage) process(s *scpImp) error {
	s.l.Printf("received E-message %s", m)

	s.dir, _ = filepath.Split(s.dir)
	s.dir = filepath.Clean(s.dir)

	ack(s.out)

	return nil
}
