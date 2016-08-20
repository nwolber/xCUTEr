package scp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

type lexReader struct {
	data  []byte
	cur   int
	pos   int
	width int
	eof   bool
}

func (l *lexReader) getData() []byte {
	var data []byte
	data, l.data = l.data[:l.cur], l.data[l.cur:]
	l.cur = 0
	return data
}

func (l *lexReader) next() (r rune, err error) {
	r, l.width = utf8.DecodeRune(l.data[l.cur:])

	if r != utf8.RuneError {
		l.cur += l.width
		l.pos++
		return r, nil
	}

	return r, io.EOF
}

func (l *lexReader) peek() (rune, error) {
	r, err := l.next()
	l.backup()
	return r, err
}

func (l *lexReader) backup() {
	l.cur -= l.width
	l.pos--
}

func (l *lexReader) ignore() {
	l.data = l.data[l.cur:]
	l.cur = 0
}

func (l *lexReader) accept(valid string) (bool, error) {
	b, err := l.next()
	if err != nil {
		return false, err
	}

	if strings.Contains(valid, string(b)) {
		return true, nil
	}
	l.backup()
	return false, nil
}

func (l *lexReader) acceptRun(valid string) error {
	for {
		b, err := l.next()
		if err != nil {
			return err
		}

		if !strings.Contains(valid, string(b)) {
			l.backup()
			break
		}
	}
	return nil
}

type stateFn func(*lexer) stateFn

type itemType int

func (i itemType) String() string {
	switch i {
	case itemTyp:
		return "itemTyp"
	case itemPerm:
		return "itemPerm"
	case itemSize:
		return "itemSize"
	case itemName:
		return "itemName"
	case itemEnd:
		return "itemEnd"
	case itemError:
		return "itemError"

	default:
		return "Unknown item type"
	}
}

const (
	itemTyp itemType = iota
	itemPerm
	itemSize
	itemName
	itemEnd
	itemError
)

type item struct {
	itemType
	val string
	pos int
}

func (i item) String() string {
	if i.itemType == itemEnd {
		return "End"
	} else if i.itemType == itemError {
		return fmt.Sprintf("<Error> %s", i.val)
	}

	return fmt.Sprintf("%q at pos %d", i.val, i.pos)
}

type lexer struct {
	in        *lexReader
	items     chan item
	recursive bool
}

func lex(b []byte) (*lexer, chan item) {
	l := &lexer{
		in: &lexReader{
			data: b,
		},
		items: make(chan item),
	}
	go l.run()
	return l, l.items
}

func (l *lexer) run() {
	for state := lexType; state != nil; {
		state = state(l)
	}
	close(l.items)
}

func (l *lexer) error(err error) {
	if err == io.EOF {
		return
	}

	l.items <- item{
		itemType: itemError,
		val:      fmt.Sprintf("lex: %s", err),
		pos:      l.in.pos,
	}
}

func (l *lexer) emit(t itemType) {
	l.items <- item{
		itemType: t,
		val:      string(l.in.getData()),
		pos:      l.in.pos,
	}
}

func lexType(l *lexer) stateFn {
	valid := "C"

	if l.recursive {
		valid += "DE"
	}

	r, err := l.in.next()

	if err != nil {
		l.error(err)
		return nil
	}

	if l.recursive {
		switch r {
		case 'D':
			fallthrough
		case 'C':
			l.emit(itemTyp)
			return lexPermissions
		case 'E':
			l.emit(itemTyp)
			return lexEnd
		default:
			l.error(fmt.Errorf("invalid token, expected: %q, found: %q at %d", "CDE", r, l.in.pos))
			return nil
		}
	} else if r == 'C' {
		l.emit(itemTyp)
		return lexPermissions
	} else {
		l.error(fmt.Errorf("invalid token, expected: %q, found: %q at %d", "C", r, l.in.pos))
	}

	return nil
}

func lexPermissions(l *lexer) stateFn {
	valid := "01234567"

	if ok, err := l.in.accept(valid); err != nil {
		l.error(err)
		return nil
	} else if !ok {
		l.error(fmt.Errorf("invalid token, expected: %q at %d", valid, l.in.pos))
		return nil
	}

	l.in.acceptRun(valid)
	l.emit(itemPerm)

	return lexSize
}

func lexSize(l *lexer) stateFn {
	if r, err := l.in.next(); err != nil {
		l.error(err)
		return nil
	} else if r != ' ' {
		l.error(fmt.Errorf("invalid token, expected: %q, found: %q at %d", ' ', r, l.in.pos))
		return nil
	} else {
		l.in.ignore()
	}

	valid := "0123456789"

	if ok, err := l.in.accept(valid); err != nil {
		l.error(err)
		return nil
	} else if !ok {
		l.error(fmt.Errorf("invalid token, expected: %q at %d", valid, l.in.pos))
	}

	l.in.acceptRun(valid)
	l.emit(itemSize)

	return lexName
}

func lexName(l *lexer) stateFn {
	if r, err := l.in.next(); err != nil {
		l.error(err)
		return nil
	} else if r != ' ' {
		l.error(fmt.Errorf("invalid token, expected: %q, found: %q at %d", ' ', r, l.in.pos))
		return nil
	} else {
		l.in.ignore()
	}

	r, err := l.in.next()
	if err != nil {
		l.error(err)
		return nil
	}

	if !isPathCharacter(r) {
		l.error(fmt.Errorf("lexer: expected a file name character, found: %q", r))
		return nil
	}

	for {
		r, err := l.in.next()
		if err != nil {
			l.error(err)
			return nil
		}

		if !isPathCharacter(r) {
			l.in.backup()
			break
		}
	}

	l.emit(itemName)

	return lexEnd
}

func isPathCharacter(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == os.PathSeparator || r == '.'
}

func lexEnd(l *lexer) stateFn {
	r, err := l.in.next()
	if err != nil {
		l.error(err)
	} else if r != '\n' {
		l.error(fmt.Errorf("invalid token, expected: %q, found: %q at %d", '\n', r, l.in.pos))
	} else {
		l.emit(itemEnd)
	}

	return nil
}
