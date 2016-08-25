// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import (
	"io"
	"testing"
	"unicode/utf8"
)

func TestLexReaderNextEmpty(t *testing.T) {
	reader := &lexReader{
		data: []byte(""),
	}

	r, err := reader.next()

	expect(t, utf8.RuneError, r)
	expect(t, io.EOF, err)

}

func TestLexReaderNext(t *testing.T) {
	reader := &lexReader{
		data: []byte("test"),
	}

	r, err := reader.next()

	expect(t, 't', r)
	expect(t, nil, err)
	expect(t, utf8.RuneLen('t'), reader.cur)
	expect(t, 1, reader.pos)
	expect(t, utf8.RuneLen('t'), reader.width)

	d := reader.getData()
	expect(t, utf8.RuneLen('t'), len(d))
	expect(t, uint8('t'), d[0])

	r, err = reader.next()
	expect(t, 'e', r)
	expect(t, nil, err)

	r, err = reader.next()
	expect(t, 's', r)
	expect(t, nil, err)

	r, err = reader.next()
	expect(t, 't', r)
	expect(t, nil, err)

	r, err = reader.next()
	expect(t, utf8.RuneError, r)
	expect(t, io.EOF, err)
}

func TestLexReaderBackup(t *testing.T) {
	reader := &lexReader{
		data: []byte("test"),
	}

	reader.next()
	reader.backup()

	d := reader.getData()
	expect(t, 0, len(d))

	r, err := reader.next()

	expect(t, 't', r)
	expect(t, nil, err)
	expect(t, utf8.RuneLen('t'), reader.cur)
	expect(t, utf8.RuneLen('t'), reader.width)

	d = reader.getData()
	expect(t, utf8.RuneLen('t'), len(d))
	expect(t, uint8('t'), d[0])
}

// func TestLexReaderIgnore(t *testing.T) {
// 	reader := &lexReader{
// 		data: []byte("test"),
// 	}

// 	reader.next()
// 	reader.ignore()
// 	reader.next()

// 	d := reader.getData()
// 	expect(t, utf8.RuneLen('e'), len(d))
// 	expect(t, uint8('e'), d[0])
// }

// func TestLexer(t *testing.T) {
// 	_, c := lex([]byte("C0664 19 index.html\n"))
// 	want := []item{
// 		item{itemTyp, "C", 1},
// 		item{itemPerm, "0664", 5},
// 		item{itemSize, "19", 8},
// 		item{itemName, "index.html", 19},
// 		item{itemEnd, "\n", 20},
// 	}

// 	for _, item := range want {
// 		i := <-c
// 		expect(t, item, i)
// 	}
// }

func TestLexerNew(t *testing.T) {
	_, c := lex([]byte("C0664 19 index.html\n"))
	want := []lexItem{
		{itemTyp, "C", 1},
		{itemNumber, "0664", 5},
		{itemSpace, " ", 6},
		{itemNumber, "19", 8},
		{itemSpace, " ", 9},
		{itemName, "index.html", 19},
		{itemEnd, "\n", 20},
	}

	for _, item := range want {
		i := <-c
		expect(t, item, i)
	}
}
