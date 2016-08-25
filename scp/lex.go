// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

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

func (l *lexReader) backup() {
	l.cur -= l.width
	l.pos--
}

func (l *lexReader) acceptFn(accept acceptFn) error {
	for {
		b, err := l.next()
		if err != nil {
			return err
		}

		if !accept(b) {
			l.backup()
			break
		}
	}
	return nil
}

type itemType int

func (i itemType) String() string {
	switch i {
	case itemTyp:
		return "itemTyp"
	case itemName:
		return "itemName"
	case itemNumber:
		return "itemNumber"
	case itemSpace:
		return "itemSpace"
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
	itemName
	itemNumber
	itemSpace
	itemEnd
	itemError
)

type lexItem struct {
	itemType
	val string
	pos int
}

func (i lexItem) String() string {
	if i.itemType == itemEnd {
		return "End"
	} else if i.itemType == itemError {
		return fmt.Sprintf("<Error> %s", i.val)
	}

	return fmt.Sprintf("<%s> %q at pos %d", i.itemType, i.val, i.pos)
}

type lexer struct {
	in    *lexReader
	items chan lexItem
}

func (l *lexer) error(err error) {
	if err == io.EOF {
		return
	}

	l.items <- lexItem{
		itemType: itemError,
		val:      fmt.Sprintf("lex: %s", err),
		pos:      l.in.pos,
	}
}

func (l *lexer) emit(t itemType) {
	l.items <- lexItem{
		itemType: t,
		val:      string(l.in.getData()),
		pos:      l.in.pos,
	}
}

func lex(b []byte) (*lexer, chan lexItem) {
	l := &lexer{
		in: &lexReader{
			data: b,
		},
		items: make(chan lexItem),
	}
	go l.run()
	return l, l.items
}

func (l *lexer) run() {
	defer close(l.items)

outer:
	for states := start(l); states != nil; {
		r, err := l.in.next()
		if err != nil {
			l.error(err)
			return
		}

		for _, s := range states {
			if s.acceptFn(r) {
				states = s.stateFn(l)
				continue outer
			}
		}

		l.error(fmt.Errorf("unexpected rune: %q", r))
		return
	}
}

type acceptFn func(r rune) bool

func acceptString(valid string) acceptFn {
	return func(r rune) bool {
		return strings.ContainsRune(valid, r)
	}
}

type stateFn func(l *lexer) []state

type state struct {
	acceptFn
	stateFn
}

func states(states ...state) []state {
	return states
}

func start(l *lexer) []state {
	valid := "CTDE"
	return states(state{
		acceptFn: acceptString(valid),
		stateFn:  lexType,
	})
}

func lexType(l *lexer) []state {
	l.emit(itemTyp)
	return states(
		number(),
		end(),
	)
}

func number() state {
	return state{
		acceptFn: unicode.IsNumber,
		stateFn:  lexNumber,
	}
}

func lexNumber(l *lexer) []state {
	if err := l.in.acceptFn(unicode.IsNumber); err != nil {
		l.error(err)
	}

	l.emit(itemNumber)
	return states(
		space(),
		end(),
	)
}

func space() state {
	return state{
		acceptFn: acceptString(" "),
		stateFn:  lexSpace,
	}
}

func lexSpace(l *lexer) []state {
	l.emit(itemSpace)
	return states(
		number(),
		name(),
	)
}

func isPathCharacter(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || r == os.PathSeparator || r == '.'
}

func name() state {
	return state{
		acceptFn: isPathCharacter,
		stateFn:  lexName,
	}
}

func lexName(l *lexer) []state {
	if err := l.in.acceptFn(isPathCharacter); err != nil {
		l.error(err)
		return nil
	}
	l.emit(itemName)

	return states(
		end(),
	)
}

func end() state {
	return state{
		acceptFn: acceptString("\n"),
		stateFn:  lexEnd,
	}
}

func lexEnd(l *lexer) []state {
	l.emit(itemEnd)
	return nil
}
