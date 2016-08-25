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

type nopWriteCloser struct {
	io.Reader
}

func (*nopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *nopWriteCloser) Read(p []byte) (int, error) {
	return w.Reader.Read(p)
}

func (*nopWriteCloser) Close() error {
	return nil
}

type directory struct {
	pathName string
	info     FileInfo
	files    []directory
	contents string
}

type file struct {
	name         string
	aTime, mTime time.Time
	isDir        bool
	mode         os.FileMode
	size         int64
}

func (f *file) Name() string {
	return f.name
}

func (f *file) AccessTime() time.Time {
	return f.aTime
}

func (f *file) ModTime() time.Time {
	return f.mTime
}

func (f *file) IsDir() bool {
	return f.isDir
}

func (f *file) Mode() os.FileMode {
	return f.mode
}

func (f *file) Size() int64 {
	return f.size
}

func (f *file) Sys() interface{} {
	panic("should never be called")
}

func TestSimpleTransfer(t *testing.T) {
	const (
		fileMode     = os.FileMode(0664)
		fileName     = "file.txt"
		fileContents = "This is a test\n"
	)
	var (
		aTime = time.Date(1989, 2, 15, 0, 0, 0, 0, time.FixedZone("CET", 3600))
		mTime = time.Date(2011, 6, 2, 0, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	)

	tests := []struct {
		in, out, cmd string
	}{
		{
			cmd: fmt.Sprintf("scp -f %s", fileName),
			in:  strings.Repeat("\x00", 3),
			out: fmt.Sprintf("C%04o %d %s\n%s\x00", fileMode, len(fileContents), fileName, fileContents),
		},
		{
			cmd: fmt.Sprintf("scp -f -p %s", fileName),
			in:  strings.Repeat("\x00", 4),
			out: fmt.Sprintf("T%d 0 %d 0\nC%04o %d %s\n%s\x00", mTime.Unix(), aTime.Unix(), fileMode, len(fileContents), fileName, fileContents),
		},
	}

	for _, tt := range tests {
		var out bytes.Buffer
		s, err := scp(tt.cmd, bytes.NewBufferString(tt.in), &out)
		expect(t, nil, err)

		s.openFile = func(name string, flags int, mode os.FileMode) (io.ReadWriteCloser, error) {
			path, err := filepath.Abs(fileName)
			if err != nil {
				t.Fatal(err)
			}

			expect(t, path, name)
			return &nopWriteCloser{
				bytes.NewBufferString(fileContents),
			}, nil
		}
		s.stat = func(name string) (FileInfo, error) {
			var f file
			f.name = fileName
			f.mode = fileMode
			f.size = int64(len(fileContents))
			f.aTime = aTime
			f.mTime = mTime
			return &f, nil
		}
		s.readDir = nil
		s.chtimes = nil

		err = s.run()
		expect(t, nil, err)
		expect(t, tt.out, out.String())
	}
}

func TestRecursiveTransfer(t *testing.T) {
	const (
		firstFileContents  = "This is the first file\n"
		secondFileContents = "This is the second file"
		fileName           = "mydir"
	)

	var (
		aTime = time.Date(1989, 2, 15, 0, 0, 0, 0, time.FixedZone("CET", 3600))
		mTime = time.Date(2011, 6, 2, 0, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	)

	dir := directory{
		pathName: fileName,
		info: &file{
			name:  fileName,
			aTime: aTime,
			mTime: mTime,
			isDir: true,
			mode:  os.FileMode(0755) | os.ModeDir,
		},
		files: []directory{
			{
				pathName: fileName + "/file1.txt",
				info: &file{
					name:  "file1.txt",
					aTime: aTime,
					mTime: mTime,
					mode:  os.FileMode(0777),
					size:  int64(len(firstFileContents)),
				},
				contents: firstFileContents,
			},
			{
				pathName: fileName + "/nestedDir",
				info: &file{
					name:  "nestedDir",
					aTime: aTime,
					mTime: mTime,
					isDir: true,
					mode:  os.FileMode(0555) | os.ModeDir,
				},
				files: []directory{
					{
						pathName: fileName + "/nestedDir/file2.txt",
						info: &file{
							name:  "file2.txt",
							aTime: aTime,
							mTime: mTime,
							mode:  os.FileMode(0511),
							size:  int64(len(secondFileContents)),
						},
						contents: secondFileContents,
					},
				},
			},
		},
	}

	tests := []struct {
		in, out, cmd string
	}{
		{
			cmd: fmt.Sprintf("scp -f -r %s", fileName),
			in:  strings.Repeat("\x00", 9),
			out: fmt.Sprintf("D%04o %d %s\n", dir.info.Mode()&os.ModePerm, dir.info.Size(), dir.info.Name()) +
				fmt.Sprintf("C%04o %d %s\n", dir.files[0].info.Mode(), dir.files[0].info.Size(), dir.files[0].info.Name()) +
				firstFileContents + "\x00" +
				fmt.Sprintf("D%04o %d %s\n", dir.files[1].info.Mode()&os.ModePerm, dir.files[1].info.Size(), dir.files[1].info.Name()) +
				fmt.Sprintf("C%04o %d %s\n", dir.files[1].files[0].info.Mode(), dir.files[1].files[0].info.Size(), dir.files[1].files[0].info.Name()) +
				secondFileContents + "\x00" +
				"E\nE\n",
		},
		{
			cmd: fmt.Sprintf("scp -f -p -r %s", fileName),
			in:  strings.Repeat("\x00", 13),
			out: fmt.Sprintf("T%d 0 %d 0\n", mTime.Unix(), aTime.Unix()) +
				fmt.Sprintf("D%04o %d %s\n", dir.info.Mode()&os.ModePerm, dir.info.Size(), dir.info.Name()) +
				fmt.Sprintf("T%d 0 %d 0\n", mTime.Unix(), aTime.Unix()) +
				fmt.Sprintf("C%04o %d %s\n", dir.files[0].info.Mode(), dir.files[0].info.Size(), dir.files[0].info.Name()) +
				firstFileContents + "\x00" +
				fmt.Sprintf("T%d 0 %d 0\n", mTime.Unix(), aTime.Unix()) +
				fmt.Sprintf("D%04o %d %s\n", dir.files[1].info.Mode()&os.ModePerm, dir.files[1].info.Size(), dir.files[1].info.Name()) +
				fmt.Sprintf("T%d 0 %d 0\n", mTime.Unix(), aTime.Unix()) +
				fmt.Sprintf("C%04o %d %s\n", dir.files[1].files[0].info.Mode(), dir.files[1].files[0].info.Size(), dir.files[1].files[0].info.Name()) +
				secondFileContents + "\x00" +
				"E\nE\n",
		},
	}

	for _, tt := range tests {
		var out bytes.Buffer
		s, err := scp(tt.cmd, bytes.NewBufferString(tt.in), &out)
		expect(t, nil, err)

		s.openFile = func(name string, flags int, mode os.FileMode) (io.ReadWriteCloser, error) {
			var contents string
			if strings.HasSuffix(name, dir.files[0].pathName) {
				contents = dir.files[0].contents
			} else if strings.HasSuffix(name, dir.files[1].files[0].pathName) {
				contents = dir.files[1].files[0].contents
			} else {
				t.Log("openFile:", name)
				return nil, errors.New("test openFile error")
			}

			return &nopWriteCloser{
				bytes.NewBufferString(contents),
			}, nil
		}
		s.stat = func(name string) (FileInfo, error) {
			if strings.HasSuffix(name, dir.pathName) {
				return dir.info, nil
			} else if strings.HasSuffix(name, dir.files[0].pathName) {
				return dir.files[0].info, nil
			} else if strings.HasSuffix(name, dir.files[1].pathName) {
				return dir.files[1].info, nil
			} else if strings.HasSuffix(name, dir.files[1].files[0].pathName) {
				return dir.files[1].files[0].info, nil
			}

			t.Log("stat:", name)
			return &fileInfo{}, errors.New("test stat error")
		}
		s.readDir = func(name string) ([]FileInfo, error) {
			if strings.HasSuffix(name, dir.pathName) {
				return []FileInfo{
					dir.files[0].info,
					dir.files[1].info,
				}, nil
			} else if strings.HasSuffix(name, dir.files[1].pathName) {
				return []FileInfo{
					dir.files[1].files[0].info,
				}, nil
			}

			t.Log("readDir:", name)
			return nil, errors.New("test readDir error")
		}
		s.chtimes = nil

		err = s.run()
		expect(t, nil, err)
		expect(t, tt.out, out.String())
	}
}
