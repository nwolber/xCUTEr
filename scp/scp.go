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
)

const (
	msgTypeC = "C"
	msgTypeD = "D"
	msgTypeE = "E"
)

var (
	errInvalidLength = errors.New("invalid length")
	// errInvalidRune   = errors.New("invalid rune")
)

func args(command string) (name string, recursive, transfer, source, verbose bool, err error) {
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
	err = set.Parse(parts[1:])
	name = set.Arg(0)
	return
}

type scpImp struct {
	name, dir                        string
	sink, source, verbose, recursive bool
	in                               *bufio.Reader
	out                              io.Writer
	l                                *log.Logger
	openFile                         func(name string, flag int, perm os.FileMode) (io.WriteCloser, error)
	mkdir                            func(name string, perm os.FileMode) error
}

func New(command string, in io.Reader, out io.Writer) error {
	s, err := scp(command, in, out)
	if err != nil {
		return err
	}
	return s.run()
}

func scp(command string, stdin io.Reader, stdout io.Writer) (*scpImp, error) {
	var (
		s   scpImp
		err error
	)
	s.name, s.recursive, s.sink, s.source, s.verbose, err = args(command)
	if err != nil {
		return nil, err
	}

	path, err := filepath.Abs(s.name)
	if err != nil {
		return nil, err
	}
	s.dir = path
	s.in = bufio.NewReader(stdin)
	s.out = stdout

	s.openFile = func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
		return os.OpenFile(name, flag, perm)
	}
	s.mkdir = os.Mkdir

	output := ioutil.Discard

	if s.verbose {
		output = os.Stderr
	}

	s.l = log.New(output, "scp", log.Flags())

	return &s, nil
}

func (s *scpImp) run() error {
	var err error
	if s.sink {
		err = s.runSink()
	}

	if err != nil {
		fmt.Fprintf(s.out, "\x01%s", err)
	}
	return err
}

type scpMessage struct {
	typ      string
	fileMode os.FileMode
	length   uint64
	fileName string
}

func (msg scpMessage) String() string {
	return fmt.Sprintf("%s%04o %d %s\n", msg.typ, uint32(msg.fileMode), msg.length, msg.fileName)
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
