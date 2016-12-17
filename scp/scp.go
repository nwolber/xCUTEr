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
	set.BoolVar(&recursive, "r", false, "recursive")
	set.BoolVar(&transfer, "t", false, "sink mode")
	set.BoolVar(&source, "f", false, "source mode")
	set.BoolVar(&verbose, "v", false, "verbose")
	set.BoolVar(&times, "p", false, "include access and modification timestamps")
	set.Bool("d", false, "dummy value, currently ignored")
	err = set.Parse(parts[1:])

	name = set.Arg(0)
	return
}

type scpImp struct {
	name, dir        string
	sink, source     bool
	times, recursive bool
	verbose          bool
	in               *bufio.Reader
	out              io.Writer
	l                logger
	mTime, aTime     time.Time
	timeSet          bool
	openFile         func(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
	mkdir            func(name string, perm os.FileMode) error
	chtimes          func(name string, aTime, mTime time.Time) error
	stat             func(name string) (FileInfo, error)
	readDir          func(name string) ([]FileInfo, error)
}

// New starts a new SCP file transfer.
func New(command string, in io.Reader, out io.Writer, verbose bool) error {
	s, err := scp(command, in, out, verbose)
	if err != nil {
		return err
	}
	return s.run()
}

func scp(command string, in io.Reader, out io.Writer, verbose bool) (*scpImp, error) {
	var (
		s   scpImp
		err error
	)
	s.name, s.recursive, s.sink, s.source, s.verbose, s.times, err = args(command)
	if err != nil {
		return nil, err
	}

	s.verbose = s.verbose || verbose

	path, err := filepath.Abs(s.name)
	if err != nil {
		return nil, err
	}
	s.dir = path
	s.in = bufio.NewReader(in)
	s.out = out

	s.openFile = openFile
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

type fileInfo struct {
	os.FileInfo
}

func (f *fileInfo) AccessTime() time.Time {
	return atime.Get(f.FileInfo)
}

// FileInfo provides informations about a file system object,
// like files or directories.
type FileInfo interface {
	os.FileInfo
	AccessTime() time.Time
}

func openFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return os.OpenFile(name, flag, perm)
}

func stat(name string) (FileInfo, error) {
	f, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	fi := fileInfo{
		FileInfo: f,
	}
	return &fi, nil
}

func readDir(name string) ([]FileInfo, error) {
	f, err := ioutil.ReadDir(name)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, len(f))
	for i := 0; i < len(f); i++ {
		files[i] = &fileInfo{
			FileInfo: f[i],
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
