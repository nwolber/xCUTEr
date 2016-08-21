// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"fmt"
	"strconv"
	"time"
)

type scpTMessage struct {
	aTime, mTime time.Time
}

func (m scpTMessage) String() string {
	return fmt.Sprintf("T%d 0 %d 0\n", m.mTime.Unix(), m.aTime.Unix())
}

func (m *scpTMessage) binders() []binder {
	return []binder{
		// mTime
		binder{itemNumber, func(val string) (err error) {
			m.mTime, err = parseUnix(val)
			return
		}},
		binder{itemSpace, nopBind},
		// mTime nanoseconds, always 0
		binder{itemNumber, nopBind},
		binder{itemSpace, nopBind},
		// aTime
		binder{itemNumber, func(val string) (err error) {
			m.aTime, err = parseUnix(val)
			return
		}},
		binder{itemSpace, nopBind},
		// aTime nanoseconds, always 0
		binder{itemNumber, nopBind},
		binder{itemEnd, nopBind},
	}
}

func (m scpTMessage) process(s *scpImp) error {
	s.l.Printf("received T-message %s", m)

	s.aTime = m.aTime
	s.mTime = m.mTime
	s.timeSet = true

	ack(s.out)
	return nil
}

func parseUnix(val string) (t time.Time, err error) {
	unix, err := strconv.ParseInt(val, 10, 64)
	t = time.Unix(unix, 0)
	return
}
