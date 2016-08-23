// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

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
	"time"
)

type nopReadCloser struct {
	io.Writer
}

func (w *nopReadCloser) Write(p []byte) (int, error) {
	return w.Writer.Write(p)
}

func (w *nopReadCloser) Read(p []byte) (int, error) {
	return 0, nil
}

func (*nopReadCloser) Close() error {
	return nil
}

func TestTransfer(t *testing.T) {
	const (
		fileMode     = os.FileMode(0664)
		fileName     = "index.html"
		fileContents = "Hello remote tunnel"
	)
	var (
		aTime = time.Date(1989, 2, 15, 0, 0, 0, 0, time.FixedZone("CET", 3600))
		mTime = time.Date(2011, 6, 2, 0, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	)

	tests := []struct {
		msg, cmd, out string
	}{
		{
			cmd: "scp -t .",
			msg: fmt.Sprintf("C%04o %d %s\n%s\x00", int(fileMode), len(fileContents), fileName, fileContents),
			out: strings.Repeat("\x00", 3),
		},
		{
			cmd: "scp -t -p .",
			msg: fmt.Sprintf("T%d 0 %d 0\nC%04o %d %s\n%s\x00", mTime.Unix(), aTime.Unix(), int(fileMode), len(fileContents), fileName, fileContents),
			out: strings.Repeat("\x00", 4),
		},
	}

	for _, tt := range tests {
		var out, file bytes.Buffer
		t.Logf("%q", tt.msg)
		in := bytes.NewBufferString(tt.msg)

		s, _ := scp(tt.cmd, in, &out)
		s.openFile = func(name string, flags int, mode os.FileMode) (readWriteCloser, error) {
			path, err := filepath.Abs(fileName)
			if err != nil {
				t.Fatal(err)
			}
			expect(t, path, name)
			expect(t, mode, fileMode)
			return &nopReadCloser{&file}, nil
		}
		s.chtimes = func(name string, a, m time.Time) error {
			if !aTime.Equal(a) {
				t.Errorf("want: %s, got: %s", aTime, a)
			}

			if !mTime.Equal(m) {
				t.Errorf("want: %s, got: %s", mTime, m)
			}
			return nil
		}
		s.mkdir = nil
		s.run()

		expect(t, tt.out, out.String())
		expect(t, "Hello remote tunnel", file.String())
	}
}

func TestTransferError(t *testing.T) {
	errorMessage := errors.New("test error")

	var out bytes.Buffer
	input := "C0664 19 index.html\nHello remote tunnel\x00"
	t.Logf("%q", input)
	in := bytes.NewBufferString(input)

	s, _ := scp("scp -t .", in, &out)
	s.openFile = func(name string, flags int, mode os.FileMode) (readWriteCloser, error) {
		return nil, errorMessage
	}
	s.mkdir = nil
	s.run()

	expect(t, "\x00\x02test error\n", out.String())
}

func TestTransferRecursive(t *testing.T) {
	var (
		aTime = time.Date(1989, 2, 15, 0, 0, 0, 0, time.FixedZone("CET", 3600))
		mTime = time.Date(2011, 6, 2, 0, 0, 0, 0, time.FixedZone("CEST", 2*3600))

		ti = scpTMessage{
			aTime: aTime,
			mTime: mTime,
		}

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
	)

	tests := []struct {
		cmd, msg, out string
	}{
		{
			cmd: "scp -t -r -p ../test",
			msg: ti.String() +
				dir1.String() +
				ti.String() +
				file1.String() +
				"This is the first file\n\x00" +
				ti.String() +
				dir2.String() +
				ti.String() +
				file2.String() +
				"This is the second file\n\x00" +
				"E\nE\n",
			out: strings.Repeat("\x00", 13),
		},
		{
			cmd: "scp -t -r ../test",
			msg: dir1.String() +
				file1.String() +
				"This is the first file\n\x00" +
				dir2.String() +
				file2.String() +
				"This is the second file\n\x00" +
				"E\nE\n",
			out: strings.Repeat("\x00", 9),
		},
	}

	for _, tt := range tests {
		t.Logf("%q", tt.msg)
		in := bytes.NewBufferString(tt.msg)

		var out, file bytes.Buffer
		s, _ := scp(tt.cmd, in, &out)
		files := []struct {
			name string
			mode os.FileMode
		}{
			{"../test/myDir/" + file1.name, file1.mode},
			{"../test/myDir/nestedDir/" + file2.name, file2.mode},
		}
		i := 0
		s.openFile = func(name string, flags int, mode os.FileMode) (readWriteCloser, error) {
			path, err := filepath.Abs(files[i].name)
			if err != nil {
				t.Fatal(err)
			}

			expect(t, path, name)
			expect(t, files[i].mode, mode)
			i++
			return &nopReadCloser{&file}, nil
		}
		s.mkdir = func(name string, perm os.FileMode) error {
			return nil
		}
		s.chtimes = func(name string, a, m time.Time) error {
			if !aTime.Equal(a) {
				t.Errorf("want: %s, got: %s", aTime, a)
			}

			if !mTime.Equal(m) {
				t.Errorf("want: %s, got: %s", mTime, m)
			}
			return nil
		}
		s.run()

		expect(t, tt.out, out.String())
		expect(t, "This is the first file\nThis is the second file\n", file.String())
	}
}

func TestShortCMessage(t *testing.T) {
	_, err := parseSCPMessage([]byte("C"))
	if err != io.EOF {
		t.Errorf("want: %s, got: %s", io.EOF, err)
	}
}

func TestInvalidCMessageType(t *testing.T) {
	_, err := parseSCPMessage([]byte("K1234 19 index.html"))
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestFileModeTooLong(t *testing.T) {
	_, err := parseSCPMessage([]byte("C1212334 19 index.html"))
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestInvalidFileMode(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0999 19 index.html"))
	if err == nil {
		t.Error("want: error, got: nil")
	}
}

func TestHappyPath(t *testing.T) {
	msg, err := parseSCPMessage([]byte("C0666 19 index.html\n"))
	m := msg.(*scpCMessage)
	expect(t, nil, err)
	expect(t, os.FileMode(0666), m.mode)
	expect(t, uint64(19), m.length)
	expect(t, "index.html", m.name)
}

func TestHappyPathEMessage(t *testing.T) {
	msg, err := parseSCPMessage([]byte("E\n"))
	_, ok := msg.(*scpEMessage)
	expect(t, true, ok)
	expect(t, nil, err)
}

func TestLongEMessage(t *testing.T) {
	_, err := parseSCPMessage([]byte("E0666 19 index.html\n"))
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestMissingLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 index.html\n"))
	if err == nil {
		t.Error("expected: error, got: nil")
	}
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
	if err == nil {
		t.Error("expected: error, got: nil")
	}
}

func TestNegativeLength(t *testing.T) {
	_, err := parseSCPMessage([]byte("C0666 -200 index.html\n"))
	if err == nil {
		t.Error("want: error, got: nil")
	}
}
