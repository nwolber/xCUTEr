// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProcessDMessageHappyPath(t *testing.T) {
	const (
		fileName = "myDir"
		fileMode = os.FileMode(0222)
	)

	var (
		recordedName string
		recordedPerm os.FileMode
	)

	var output bytes.Buffer
	s, _ := scp("scp -t -r .", nil, &output)
	s.mkdir = func(name string, perm os.FileMode) error {
		recordedName = name
		recordedPerm = perm
		return nil
	}

	msg := scpDMessage{
		mode:   fileMode,
		length: 0,
		name:   fileName,
	}

	path, err := filepath.Abs(fileName)
	if err != nil {
		t.Fatal(err)
	}

	err = msg.process(s)
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, path, recordedName)
	expect(t, fileMode, recordedPerm)
	expect(t, "\x00", output.String())
}

func TestProcessDMessageHappyPath2(t *testing.T) {
	const (
		wantFileName = "test/myDir"
		filePerm     = os.FileMode(0666)
	)

	var (
		recordedName string
		recordedPerm os.FileMode
	)

	var output bytes.Buffer
	s, _ := scp("scp -t -r test", nil, &output)
	s.openFile = nil
	s.mkdir = func(name string, perm os.FileMode) error {
		recordedName = name
		recordedPerm = perm
		return nil
	}

	msg := scpDMessage{
		mode:   filePerm,
		length: 0,
		name:   "myDir",
	}

	path, err := filepath.Abs(wantFileName)
	if err != nil {
		t.Fatal(err)
	}

	err = msg.process(s)
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, path, recordedName)
	expect(t, filePerm, recordedPerm)
	expect(t, "\x00", output.String())
}

func TestProcessDMessageInvalidLength(t *testing.T) {
	var output bytes.Buffer
	s, _ := scp("scp -t -r .", nil, &output)
	msg := scpDMessage{
		mode:   os.FileMode(0222),
		length: 42,
		name:   "test",
	}
	err := msg.process(s)
	expect(t, errInvalidLength, err)
	expect(t, "", output.String())
}

func TestProcessEMessageHappyPath(t *testing.T) {
	const (
		name = "test"
	)

	var output bytes.Buffer
	s, _ := scp("scp -t -r "+name, nil, &output)
	s.dir, _ = filepath.Abs("test/mydir")
	s.openFile = nil
	s.mkdir = nil

	path, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}

	err = scpEMessage{}.process(s)
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, "\x00", output.String())
}

func TestProcessEMessageHappyPath2(t *testing.T) {
	const (
		name = "."
	)

	var output bytes.Buffer
	s, _ := scp("scp -t -r "+name, nil, &output)
	s.dir, _ = filepath.Abs("test")
	s.openFile = nil
	s.mkdir = nil

	path, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}

	err = scpEMessage{}.process(s)
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, "\x00", output.String())
}

func TestProcessEMessageHappyPath3(t *testing.T) {
	const (
		name = "."
	)

	var output bytes.Buffer
	s, _ := scp("scp -t -r "+name, nil, &output)
	s.dir, _ = filepath.Abs(".")
	s.openFile = nil
	s.mkdir = nil

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path, _ := filepath.Split(wd)
	path = filepath.Clean(path)

	err = scpEMessage{}.process(s)
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, "\x00", output.String())
}

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
