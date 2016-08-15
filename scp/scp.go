package scp

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	errUnexpectedToken       = errors.New("unexpected token")
	errMessageTypeNotAllowed = errors.New("message type not allowed")
)

type scp struct {
	name      string
	recursive bool
	in        *bufio.Reader
	out       io.Writer
	buffer    []byte
	storeFile func(name string, flag int, perm os.FileMode) (io.WriteCloser, error)
}

func args(command string) (name string, recursive, transfer bool, err error) {
	parts := strings.SplitN(command, " ", -1)
	if len(parts) < 2 {
		err = errors.New("expected at least 'scp' and one parameter")
		return
	}

	set := flag.NewFlagSet(parts[0], flag.ContinueOnError)
	set.BoolVar(&recursive, "r", false, "")
	set.BoolVar(&transfer, "t", false, "")
	err = set.Parse(parts[1:])
	return
}

func New(command string, in io.Reader, out io.Writer) error {
	name, recursive, transfer, err := args(command)

	if err != nil {
		return err
	}

	if !transfer {
		return fmt.Errorf("unknown command %q", command)
	}

	return new(name, recursive, in, out).run()
}

func new(name string, recursive bool, in io.Reader, out io.Writer) *scp {
	return &scp{
		name:      name,
		recursive: recursive,
		in:        bufio.NewReader(in),
		out:       out,
		storeFile: func(name string, flag int, perm os.FileMode) (io.WriteCloser, error) {
			f, err := os.OpenFile(name, flag, perm)
			return f, err
		},
	}
}

func (s *scp) run() error {
	log.Println("Initiating transfer")
	err := s.transfer()

	if err != nil {
		s.out.Write([]byte("\x01" + err.Error()))
		return err
	}
	return nil
}

func (s *scp) transfer() error {
	ack(s.out)

	for {
		b, err := s.in.Peek(1)
		if err != nil {
			return err
		}

		if b[0] == byte(0) {
			break
		}

		b, err = s.in.ReadBytes('\n')
		if err != nil {
			return err
		}

		msg, err := parseSCPMessage(b)
		if err != nil {
			return err
		}

		if !s.recursive && msg.typ != "C" {
			return errMessageTypeNotAllowed
		}

		switch msg.typ {
		case "C":
			err = s.processCMessage(msg)
		}

		if err != nil {
			return err
		}

		ack(s.out)
	}

	return nil
}

func ack(out io.Writer) {
	out.Write([]byte{0})
}

type scpMessage struct {
	typ      string
	fileMode os.FileMode
	length   uint64
	fileName string
}

func (msg scpMessage) String() string {
	return fmt.Sprintf("{FileMode: %04o, Length: %d, Name: %q}", uint32(msg.fileMode), msg.length, msg.fileName)
}

func parseSCPMessage(input []byte) (scpMessage, error) {
	const (
		cStart = 'C'
	)

	var msg scpMessage

	switch input[0] {
	case 'C':
		fallthrough
	case 'D':
		fallthrough
	case 'E':
		msg.typ = string(input[0])
	default:
		return msg, errUnexpectedToken
	}

	buf := bytes.NewBuffer(input)
	b, err := buf.ReadBytes(' ')
	if err != nil {
		return msg, err
	}

	if len(b) != 6 {
		return msg, errUnexpectedToken
	}

	mode, err := strconv.ParseUint(string(b[1:5]), 8, 32)
	if err != nil {
		return msg, err
	}
	msg.fileMode = os.FileMode(mode)

	b, err = buf.ReadBytes(' ')
	if err != nil {
		return msg, err
	}

	msg.length, err = strconv.ParseUint(string(b[:len(b)-1]), 10, 64)
	if err != nil {
		return msg, err
	}

	b, err = buf.ReadBytes('\n')
	if err != nil {
		return msg, err
	}

	if len(b) <= 1 {
		return msg, io.EOF
	}

	msg.fileName = string(b[:len(b)-1])
	return msg, nil
}

func (s *scp) processCMessage(msg scpMessage) error {
	log.Printf("received C-message %s", msg)

	path := filePath(s.name, msg.fileName)
	f, err := s.storeFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, msg.fileMode)
	if err != nil {
		return err
	}
	defer f.Close()

	ack(s.out)

	if s.buffer == nil {
		s.buffer = make([]byte, 1024*1024)
	}

	var n uint64
	fileReader := io.LimitReader(s.in, int64(msg.length))
	for n < msg.length {
		read, err := fileReader.Read(s.buffer)
		if err != nil {
			return nil
		}
		for m := 0; m < read; {
			written, err := f.Write(s.buffer[:read-m])
			if err != nil {
				return err
			}
			m += written
		}

		n += uint64(read)
	}

	return nil
}

func filePath(commandPath, filePath string) string {
	if filepath.Dir(commandPath) == commandPath {
		return filepath.Join(commandPath, filePath)
	}

	return commandPath
}
