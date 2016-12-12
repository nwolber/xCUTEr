// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import "testing"

func TestBuilderCommand(t *testing.T) {
	s := &StringBuilder{}
	c := s.Command(&Command{Command: "first"}).(Stringer)

	want := "Execute \"first\""

	if got := c.String(nil); got != want {
		t.Errorf("want: %q, got: %q", want, got)
	}
}

func TestBuilderMultiple(t *testing.T) {
	s := &StringBuilder{}
	g := s.Sequential()
	g.Append(s.Command(&Command{Command: "first"}))
	g.Append(s.Command(&Command{Command: "second"}))

	want := "Sequential\n" +
		"├─ Execute \"first\"\n" +
		"└─ Execute \"second\""

	if got := g.(Stringer).String(nil); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuilderEmptyMultiple(t *testing.T) {
	s := &StringBuilder{}
	g := s.Sequential()

	want := "Sequential"

	if got := g.(Stringer).String(nil); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuilderNested(t *testing.T) {
	s := &StringBuilder{}
	g1 := s.Sequential()
	g2 := s.Parallel()
	g2.Append(s.Command(&Command{Command: "first"}))
	g2.Append(s.Command(&Command{Command: "third"}))
	g1.Append(g2.Wrap())
	g1.Append(s.Command(&Command{Command: "second"}))

	want := "Sequential\n" +
		"├─ Parallel\n" +
		"│  ├─ Execute \"first\"\n" +
		"│  └─ Execute \"third\"\n" +
		"└─ Execute \"second\""

	if got := g1.(Stringer).String(nil); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuilderNested3(t *testing.T) {
	s := &StringBuilder{}
	g1 := s.Sequential()
	g2 := s.Parallel()
	g2.Append(s.Command(&Command{Command: "second"}))
	g2.Append(s.Command(&Command{Command: "third"}))
	g1.Append(g2.Wrap())

	want := "Sequential\n" +
		"└─ Parallel\n" +
		"   ├─ Execute \"second\"\n" +
		"   └─ Execute \"third\""

	if got := g1.(Stringer).String(nil); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuilderNested4(t *testing.T) {
	s := &StringBuilder{}
	g1 := s.Sequential()
	g1.Append(s.Command(&Command{Command: "first"}))
	g1.Append(s.Command(&Command{Command: "second"}))
	g2 := s.Parallel()
	g2.Append(s.Command(&Command{Command: "third"}))
	g1.Append(g2.Wrap())

	want := "Sequential\n" +
		"├─ Execute \"first\"\n" +
		"├─ Execute \"second\"\n" +
		"└─ Parallel\n" +
		"   └─ Execute \"third\""

	if got := g1.(Stringer).String(nil); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestMax(t *testing.T) {
	s := &StringBuilder{}
	g1 := s.Sequential()
	g1.(*multiple).max = 2
	g1.Append(s.Command(&Command{Command: "first"}))
	g1.Append(s.Command(&Command{Command: "second"}))
	g1.Append(s.Command(&Command{Command: "third"}))

	want := "Sequential\n" +
		"├─ Execute \"first\"\n" +
		"├─ Execute \"second\"\n" +
		"└─ and 1 more ..."

	if got := g1.(Stringer).String(nil); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}
