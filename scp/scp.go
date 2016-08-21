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
	openFile         func(name string, flag int, perm os.FileMode) (io.WriteCloser, error)
	mkdir            func(name string, perm os.FileMode) error
	chtimes          func(name string, aTime, mTime time.Time) error
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

	s.openFile = func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
		return os.OpenFile(name, flag, perm)
	}
	s.mkdir = os.MkdirAll
	s.chtimes = os.Chtimes

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
	if s.sink {
		err = s.runSink()
	}

	if err != nil {
		fmt.Fprintf(s.out, "\x02%s", err)
	}
	return err
}

func ack(out io.Writer) {
	out.Write([]byte{0})
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
