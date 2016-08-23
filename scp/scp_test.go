// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import "testing"

func expect(t *testing.T, want, got interface{}) {
	if want != got {
		t.Errorf("want: <%T>%q, got: <%T>%q", want, want, got, got)
	}
}

func TestNoErrorOnIgnoredDFlag(t *testing.T) {
	_, recursive, sink, source, verbose, times, err := args("scp -d -f -t -p -v -r")
	expect(t, true, recursive)
	expect(t, true, sink)
	expect(t, true, source)
	expect(t, true, verbose)
	expect(t, true, times)
	expect(t, nil, err)
}

func TestFilePath(t *testing.T) {
	expect(t, "index.html", filePath(".", "index.html"))
	expect(t, "test/bla.html", filePath("test/bla.html", "index.html"))
	expect(t, "test", filePath("test", "index.html"))
	expect(t, "/test", filePath("/test", "index.html"))
}
