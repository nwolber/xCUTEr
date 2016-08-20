package scp

import "testing"

func expect(t *testing.T, want, got interface{}) {
	if want != got {
		t.Errorf("want: <%T>%q, got: <%T>%q", want, want, got, got)
	}
}

func TestFilePath(t *testing.T) {
	expect(t, "index.html", filePath(".", "index.html"))
	expect(t, "test/bla.html", filePath("test/bla.html", "index.html"))
	expect(t, "test", filePath("test", "index.html"))
	expect(t, "/test", filePath("/test", "index.html"))
}
