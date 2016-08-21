package scp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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
