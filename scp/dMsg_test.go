package scp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
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
