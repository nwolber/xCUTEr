package scp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
)

func expect(t *testing.T, want, got interface{}) {
	if want != got {
		t.Errorf("want: %q, got: %q", want, got)
	}
}

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

	s := new(".", false, in, &out)
	s.storeFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		expect(t, fileName, name)
		expect(t, mode, fileMode)
		return &nopCloser{&file}, nil
	}
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

	s := new(".", false, in, &out)
	s.storeFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		return nil, errorMessage
		// var file bytes.Buffer
		// return &nopCloser{&file}, nil
	}
	s.run()

	expect(t, "\x00\x01test error", out.String())
}

func TestShortCMessage(t *testing.T) {
	_, err := parseSCPMessage([]byte("C"))
	if err != io.EOF {
		t.Errorf("want: %s, got: %s", io.EOF, err)
	}
}

func TestInvalidCMessageType(t *testing.T) {
	_, err := parseSCPMessage([]byte("K1234 19 index.html"))
	expect(t, errUnexpectedToken, err)
}

func TestFileModeTooLong(t *testing.T) {
	_, err := parseSCPMessage([]byte("C1212334 19 index.html"))
	expect(t, errUnexpectedToken, err)
}

func TestInvalidFileMode(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0999 19 index.html"))
	if err == nil {
		t.Error("want: error, got: nil")
	}
}

func TestHappyPath(t *testing.T) {
	msg, err := parseSCPMessage([]byte("C0666 19 index.html\n"))
	expect(t, nil, err)
	expect(t, os.FileMode(0666), msg.fileMode)
	expect(t, uint64(19), msg.length)
	expect(t, "index.html", msg.fileName)
}

func TestMissingLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 index.html\n"))
	expect(t, io.EOF, err)
}

func TestInvalidLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 1x index.html"))
	if err == nil {
		t.Error("want: error, got: nil")
	}
}

func TestMissingNewLine(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 19 index.html"))
	expect(t, io.EOF, err)
}

func TestMissingFileName(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 19 \n"))
	expect(t, io.EOF, err)
}

func TestFilePath(t *testing.T) {
	expect(t, "index.html", filePath(".", "index.html"))
	expect(t, "test/bla.html", filePath("test/bla.html", "index.html"))
	expect(t, "test", filePath("test", "index.html"))
	expect(t, "/test", filePath("/test", "index.html"))
}
