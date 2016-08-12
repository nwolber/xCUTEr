package main

import "testing"

// func TestExecuteCommand(t *testing.T) {
// 	s := partExecuteCommand{cmd: &command{Command: "first"}}

// 	want := "Execute \"first\""

// 	if got := s.String(); got != want {
// 		t.Errorf("want: %q, got: %q", want, got)
// 	}
// }

// func TestMultiple(t *testing.T) {
// 	s := multiple{
// 		typ: "Sequential",
// 		stringers: []fmt.Stringer{
// 			&partExecuteCommand{cmd: &command{Command: "first"}},
// 			&partExecuteCommand{cmd: &command{Command: "second"}},
// 		},
// 	}

// 	want := "Sequential\n" +
// 		"├─ Execute \"first\"\n" +
// 		"└─ Execute \"second\""

// 	if got := s.String(); got != want {
// 		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
// 	}
// }

// func TestNestedMultiple(t *testing.T) {
// 	s := multiple{
// 		typ: "Sequential",
// 		stringers: []fmt.Stringer{
// 			&multiple{
// 				typ: "Parallel",
// 				stringers: []fmt.Stringer{
// 					&partExecuteCommand{cmd: &command{Command: "first"}},
// 					&partExecuteCommand{cmd: &command{Command: "third"}},
// 				},
// 			},
// 			&partExecuteCommand{cmd: &command{Command: "second"}},
// 		},
// 	}

// 	want := "Sequential\n" +
// 		"├─ Parallel\n" +
// 		"│  ├─ Execute \"first\"\n" +
// 		"│  └─ Execute \"third\"\n" +
// 		"└─ Execute \"second\""

// 	if got := s.String(); got != want {
// 		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
// 	}
// }

func TestBuilderCommand(t *testing.T) {
	s := newStringBuilder()
	s.Command(&command{Command: "first"})

	want := "Execute \"first\""

	if got := s.String(); got != want {
		t.Errorf("want: %q, got: %q", want, got)
	}
}

func TestBuilderMultiple(t *testing.T) {
	s := newStringBuilder()
	s.Group()
	s.Command(&command{Command: "first"})
	s.Command(&command{Command: "second"})
	s.Sequential()

	want := "Sequential\n" +
		"├─ Execute \"first\"\n" +
		"└─ Execute \"second\""

	if got := s.String(); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuilderNested(t *testing.T) {
	s := newStringBuilder()
	s.Group()
	s.Group()
	s.Command(&command{Command: "first"})
	s.Command(&command{Command: "third"})
	s.Parallel()
	s.Command(&command{Command: "second"})
	s.Sequential()

	want := "Sequential\n" +
		"├─ Parallel\n" +
		"│  ├─ Execute \"first\"\n" +
		"│  └─ Execute \"third\"\n" +
		"└─ Execute \"second\""

	if got := s.String(); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

// func TestBuilderNested2(t *testing.T) {
// 	s := newStringBuilder()
// 	s.Group()
// 	s.Command(&command{Command: "first"})
// 	s.Group()
// 	s.Command(&command{Command: "second"})
// 	s.Command(&command{Command: "third"})
// 	s.Parallel()
// 	s.Sequential()

// 	want := "Sequential\n" +
// 		"├─ Parallel\n" +
// 		"│  ├─ Execute \"second\"\n" +
// 		"│  └─ Execute \"third\"\n" +
// 		"└─ Execute \"first\""

// 	if got := s.String(); got != want {
// 		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
// 	}
// }

func TestBuilderNested3(t *testing.T) {
	s := newStringBuilder()
	s.Group()
	s.Group()
	s.Command(&command{Command: "second"})
	s.Command(&command{Command: "third"})
	s.Parallel()
	s.Sequential()

	want := "Sequential\n" +
		"└─ Parallel\n" +
		"   ├─ Execute \"second\"\n" +
		"   └─ Execute \"third\""

	if got := s.String(); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}

func TestBuilderNested4(t *testing.T) {
	s := newStringBuilder()
	s.Group()
	s.Command(&command{Command: "first"})
	s.Command(&command{Command: "second"})
	s.Group()
	// s.Command(&command{Command: "second"})
	s.Command(&command{Command: "third"})
	s.Parallel()
	s.Sequential()

	want := "Sequential\n" +
		"├─ Execute \"first\"\n" +
		"├─ Execute \"second\"\n" +
		"└─ Parallel\n" +
		// "   ├─ Execute \"second\"\n" +
		"   └─ Execute \"third\""

	if got := s.String(); got != want {
		t.Errorf("want:\n%s\n\ngot:\n%s", want, got)
	}
}
