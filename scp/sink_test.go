package scp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type nopCloser struct {
	io.Writer
}

func (w *nopCloser) Write(p []byte) (int, error) {
	return w.Writer.Write(p)
}

func (*nopCloser) Close() error {
	return nil
}

func TestTransfer(t *testing.T) {
	const (
		fileMode     = os.FileMode(0664)
		fileName     = "index.html"
		fileContents = "Hello remote tunnel"
	)

	var out, file bytes.Buffer
	input := fmt.Sprintf("C%04o %d %s\n%s\x00", int(fileMode), len(fileContents), fileName, fileContents)
	t.Logf("%q", input)
	in := bytes.NewBufferString(input)

	s, _ := scp("scp -t .", in, &out)
	s.openFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		path, err := filepath.Abs(fileName)
		if err != nil {
			t.Fatal(err)
		}
		expect(t, path, name)
		expect(t, mode, fileMode)
		return &nopCloser{&file}, nil
	}
	s.mkdir = nil
	s.run()

	expect(t, "\x00\x00\x00", out.String())
	expect(t, "Hello remote tunnel", file.String())
}

func TestTransferError(t *testing.T) {
	errorMessage := errors.New("test error")

	var out bytes.Buffer
	input := "C0664 19 index.html\nHello remote tunnel\x00"
	t.Logf("%q", input)
	in := bytes.NewBufferString(input)

	s, _ := scp("scp -t .", in, &out)
	s.openFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		return nil, errorMessage
	}
	s.mkdir = nil
	s.run()

	expect(t, "\x00\x02test error", out.String())
}

func TestTransferRecursive(t *testing.T) {
	var (
		dir1 = scpDMessage{
			mode: os.FileMode(0775),
			name: "myDir",
		}

		file1 = scpCMessage{
			mode:   os.FileMode(0664),
			length: 23,
			name:   "file1.txt",
		}

		dir2 = scpDMessage{
			mode: os.FileMode(0775),
			name: "nestedDir",
		}

		file2 = scpCMessage{
			mode:   os.FileMode(0664),
			length: 24,
			name:   "file2.txt",
		}
		out, file bytes.Buffer
	)

	input := dir1.String() +
		file1.String() +
		"This is the first file\n\x00" +
		dir2.String() +
		file2.String() +
		"This is the second file\n\x00" +
		"E\nE\n"

	t.Logf("%q", input)
	in := bytes.NewBufferString(input)

	s, _ := scp("scp -t -r ../test", in, &out)
	files := []struct {
		name string
		mode os.FileMode
	}{
		{"../test/myDir/" + file1.name, file1.mode},
		{"../test/myDir/nestedDir/" + file2.name, file2.mode},
	}
	i := 0
	s.openFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		path, err := filepath.Abs(files[i].name)
		if err != nil {
			t.Fatal(err)
		}

		expect(t, path, name)
		expect(t, files[i].mode, mode)
		i++
		return &nopCloser{&file}, nil
	}
	s.mkdir = func(name string, perm os.FileMode) error {
		return nil
	}
	s.run()

	expect(t, strings.Repeat("\x00", 9), out.String())
	expect(t, "This is the first file\nThis is the second file\n", file.String())
}

func TestShortCMessage(t *testing.T) {
	_, err := parseSCPMessage([]byte("C"), false)
	if err != io.EOF {
		t.Errorf("want: %s, got: %s", io.EOF, err)
	}
}

func TestInvalidCMessageType(t *testing.T) {
	_, err := parseSCPMessage([]byte("K1234 19 index.html"), false)
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestFileModeTooLong(t *testing.T) {
	_, err := parseSCPMessage([]byte("C1212334 19 index.html"), false)
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestInvalidFileMode(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0999 19 index.html"), false)
	if err == nil {
		t.Error("want: error, got: nil")
	}
}

func TestHappyPath(t *testing.T) {
	msg, err := parseSCPMessage([]byte("C0666 19 index.html\n"), false)
	m := msg.(*scpCMessage)
	expect(t, nil, err)
	expect(t, os.FileMode(0666), m.mode)
	expect(t, uint64(19), m.length)
	expect(t, "index.html", m.name)
}

func TestHappyPathEMessage(t *testing.T) {
	msg, err := parseSCPMessage([]byte("E\n"), true)
	_, ok := msg.(*scpEMessage)
	expect(t, true, ok)
	expect(t, nil, err)
}

func TestLongEMessage(t *testing.T) {
	_, err := parseSCPMessage([]byte("E0666 19 index.html\n"), true)
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestMissingLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 index.html\n"), false)
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestInvalidLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 1x index.html"), false)
	if err == nil {
		t.Error("want: error, got: nil")
	}
}

func TestMissingNewLine(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 19 index.html"), false)
	expect(t, io.EOF, err)
}

func TestMissingFileName(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 19 \n"), false)
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestNegativeLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 -200 index.html\n"), false)
	if err == nil {
		t.Error("want: error, got: nil")
	}
}
