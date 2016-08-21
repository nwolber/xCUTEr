// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/djherbis/atime"
)

var (
	errInvalidLength = errors.New("invalid length")
)

func args(command string) (name string, recursive, transfer, source, verbose, times bool, err error) {
	parts := strings.SplitN(command, " ", -1)
	if len(parts) < 2 {
		err = errors.New("expected at least 'scp' and one parameter")
		return
	}

	set := flag.NewFlagSet(parts[0], flag.ContinueOnError)
	set.BoolVar(&recursive, "r", false, "")
	set.BoolVar(&transfer, "t", false, "")
	set.BoolVar(&source, "f", false, "")
	set.BoolVar(&verbose, "v", false, "")
	set.BoolVar(&times, "p", false, "")
	err = set.Parse(parts[1:])
	name = set.Arg(0)
	return
}

type readWriteCloser interface {
	io.Reader
	io.Writer
	io.Closer
}

type scpImp struct {
	name, dir        string
	sink, source     bool
	times, recursive bool
	verbose          bool
	in               *bufio.Reader
	out              io.Writer
	l                *log.Logger
	mTime, aTime     time.Time
	timeSet          bool
	openFile         func(name string, flag int, perm os.FileMode) (readWriteCloser, error)
	mkdir            func(name string, perm os.FileMode) error
	chtimes          func(name string, aTime, mTime time.Time) error
	stat             func(name string) (fileInfo, error)
	readDir          func(name string) ([]fileInfo, error)
}

func New(command string, in io.Reader, out io.Writer) error {
	s, err := scp(command, in, out)
	if err != nil {
		return err
	}
	return s.run()
}

func scp(command string, in io.Reader, out io.Writer) (*scpImp, error) {
	var (
		s   scpImp
		err error
	)
	s.name, s.recursive, s.sink, s.source, s.verbose, s.times, err = args(command)
	if err != nil {
		return nil, err
	}

	path, err := filepath.Abs(s.name)
	if err != nil {
		return nil, err
	}
	s.dir = path
	s.in = bufio.NewReader(in)
	s.out = out

	s.openFile = func(name string, flag int, perm os.FileMode) (readWriteCloser, error) {
		return os.OpenFile(name, flag, perm)
	}
	s.mkdir = os.MkdirAll
	s.chtimes = os.Chtimes
	s.stat = stat
	s.readDir = readDir

	output := ioutil.Discard
	if s.verbose {
		output = os.Stderr
	}

	s.l = log.New(output, "scp ", log.Flags())
	s.l.Println(command)

	return &s, nil
}

type fileInfo struct {
	name         string
	mode         os.FileMode
	mTime, aTime time.Time
	size         int64
	isDir        bool
}

func (s *scpImp) run() error {
	var err error
	if s.sink && s.source {
		return errors.New("either -f or -t can be specified")
	}

	if s.sink {
		err = s.runSink()
	} else if s.source {
		err = s.runSource()
	} else {
		return errors.New("either -f or -t has to be specified")
	}

	if err != nil {
		fmt.Fprintf(s.out, "\x02%s\n", err)
	}
	return err
}

func stat(name string) (fileInfo, error) {
	f, err := os.Stat(name)
	var fi fileInfo
	if err != nil {
		return fi, err
	}
	fi.aTime = atime.Get(f)

	fi.mode = f.Mode()
	fi.mTime = f.ModTime()
	fi.name = f.Name()
	fi.size = f.Size()
	fi.isDir = f.IsDir()
	return fi, nil
}

func readDir(name string) ([]fileInfo, error) {
	f, err := ioutil.ReadDir(name)
	if err != nil {
		return nil, err
	}

	files := make([]fileInfo, len(f))
	for i := 0; i < len(f); i++ {
		fi := f[i]
		files[i] = fileInfo{
			aTime: atime.Get(fi),
			isDir: fi.IsDir(),
			mode:  fi.Mode(),
			mTime: fi.ModTime(),
			name:  fi.Name(),
			size:  fi.Size(),
		}
	}
	return files, nil
}

func filePath(commandPath, filePath string) string {
	if filepath.Dir(commandPath) == commandPath {
		return filepath.Join(commandPath, filePath)
	}

	return commandPath
}

type scpMessage interface {
	fmt.Stringer
	binders() []binder
	process(s *scpImp) error
}
