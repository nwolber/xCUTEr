// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"fmt"
	"testing"
	"time"
)

func TestTMessage(t *testing.T) {
	var (
		aTime = time.Date(1989, 2, 15, 0, 0, 0, 0, time.FixedZone("CET", 3600))
		mTime = time.Date(2011, 6, 2, 0, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	)
	msg := fmt.Sprintf("T%d 0 %d 0\n", mTime.Unix(), aTime.Unix())
	t.Log(msg)
	m, err := parseSCPMessage([]byte(msg))

	expect(t, nil, err)

	mm, ok := m.(*scpTMessage)

	expect(t, true, ok)

	if !aTime.Equal(mm.aTime) {
		t.Errorf("want: %s, got: %s", aTime, mm.aTime)
	}

	if !mTime.Equal(mm.mTime) {
		t.Errorf("want: %s, got: %s", mTime, mm.mTime)
	}
}
