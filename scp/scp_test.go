package scp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	s, _ := new(".", false, in, &out)
	s.openFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		expect(t, fileName, name)
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

	s, _ := new(".", false, in, &out)
	s.openFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		return nil, errorMessage
	}
	s.mkdir = nil
	s.run()

	expect(t, "\x00\x01test error", out.String())
}

func TestTransferRecursive(t *testing.T) {
	var (
		dir1 = scpMessage{
			typ:      "D",
			fileMode: os.FileMode(0775),
			fileName: "myDir",
		}

		file1 = scpMessage{
			typ:      "C",
			fileMode: os.FileMode(0664),
			length:   23,
			fileName: "file1.txt",
		}

		dir2 = scpMessage{
			typ:      "D",
			fileMode: os.FileMode(0775),
			fileName: "nestedDir",
		}

		file2 = scpMessage{
			typ:      "C",
			fileMode: os.FileMode(0664),
			length:   24,
			fileName: "file2.txt",
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

	s, _ := new(".", true, in, &out)
	s.openFile = func(name string, flags int, mode os.FileMode) (io.WriteCloser, error) {
		// expect(t, fileName, name)
		// expect(t, mode, fileMode)
		return &nopCloser{&file}, nil
	}
	s.mkdir = func(name string, perm os.FileMode) error {
		return nil
	}
	s.run()

	expect(t, "\x00\x00\x00\x00\x00\x00\x00\x00\x00", out.String())
	expect(t, "This is the first file\nThis is the second file\n", file.String())
}

func TestShortCMessage(t *testing.T) {
	_, err := parseSCPMessage([]byte("C"))
	if err != io.EOF {
		t.Errorf("want: %s, got: %s", io.EOF, err)
	}
}

func TestInvalidCMessageType(t *testing.T) {
	_, err := parseSCPMessage([]byte("K1234 19 index.html"))
	expect(t, errInvalidToken, err)
}

func TestFileModeTooLong(t *testing.T) {
	_, err := parseSCPMessage([]byte("C1212334 19 index.html"))
	expect(t, errInvalidToken, err)
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

func TestHappyPathEMessage(t *testing.T) {
	msg, err := parseSCPMessage([]byte("E\n"))
	expect(t, nil, err)
	expect(t, msg.typ, "E")
}

func TestLongEMessage(t *testing.T) {
	_, err := parseSCPMessage([]byte("E0666 19 index.html\n"))
	expect(t, errInvalidToken, err)
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

func TestNegativeLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 -200 index.html\n"))
	if err == nil {
		t.Error("want: error, got: nil")
	}
}

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
	s, _ := new(".", true, nil, &output)
	s.mkdir = func(name string, perm os.FileMode) error {
		recordedName = name
		recordedPerm = perm
		return nil
	}

	msg := scpMessage{
		typ:      "D",
		fileMode: fileMode,
		length:   0,
		fileName: fileName,
	}

	path, err := filepath.Abs(fileName)
	if err != nil {
		t.Fatal(err)
	}

	err = s.processDMessage(msg)
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
	s, _ := new("test", true, nil, &output)
	s.openFile = nil
	s.mkdir = func(name string, perm os.FileMode) error {
		recordedName = name
		recordedPerm = perm
		return nil
	}

	msg := scpMessage{
		typ:      "D",
		fileMode: filePerm,
		length:   0,
		fileName: "myDir",
	}

	path, err := filepath.Abs(wantFileName)
	if err != nil {
		t.Fatal(err)
	}

	err = s.processDMessage(msg)
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, path, recordedName)
	expect(t, filePerm, recordedPerm)
	expect(t, "\x00", output.String())
}

func TestProcessDMessageInvalidLength(t *testing.T) {
	var output bytes.Buffer
	s, _ := new(".", true, nil, &output)
	msg := scpMessage{
		typ:      "D",
		fileMode: os.FileMode(0222),
		length:   42,
		fileName: "test",
	}
	err := s.processDMessage(msg)
	expect(t, errInvalidLength, err)
	expect(t, "", output.String())
}

func TestProcessEMessageHappyPath(t *testing.T) {
	const (
		name = "test"
	)

	var output bytes.Buffer
	s, _ := new(name, true, nil, &output)
	s.dir, _ = filepath.Abs("test/mydir")
	s.openFile = nil
	s.mkdir = nil

	path, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}

	err = s.processEMessage(scpMessage{typ: "E"})
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, "\x00", output.String())
}

func TestProcessEMessageHappyPath2(t *testing.T) {
	const (
		name = "."
	)

	var output bytes.Buffer
	s, _ := new(name, true, nil, &output)
	s.dir, _ = filepath.Abs("test")
	s.openFile = nil
	s.mkdir = nil

	path, err := filepath.Abs(name)
	if err != nil {
		t.Fatal(err)
	}

	err = s.processEMessage(scpMessage{typ: "E"})
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, "\x00", output.String())
}

func TestProcessEMessageHappyPath3(t *testing.T) {
	const (
		name = "."
	)

	var output bytes.Buffer
	s, _ := new(name, true, nil, &output)
	s.dir, _ = filepath.Abs(".")
	s.openFile = nil
	s.mkdir = nil

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	path, _ := filepath.Split(wd)
	path = filepath.Clean(path)

	err = s.processEMessage(scpMessage{typ: "E"})
	expect(t, nil, err)
	expect(t, path, s.dir)
	expect(t, "\x00", output.String())
}

func TestFilePath(t *testing.T) {
	expect(t, "index.html", filePath(".", "index.html"))
	expect(t, "test/bla.html", filePath("test/bla.html", "index.html"))
	expect(t, "test", filePath("test", "index.html"))
	expect(t, "/test", filePath("/test", "index.html"))
}
