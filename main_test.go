package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestVersionFlag(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := run([]string{"-version"})
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	got := strings.TrimSpace(buf.String())
	want := "webmri " + version
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
