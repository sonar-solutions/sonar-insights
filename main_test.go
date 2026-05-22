package main

import (
	"bytes"
	"os"
	"testing"
)

func TestMainOutput(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	old := os.Stdout
	os.Stdout = w

	main()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	want := "Hello, World!\nGreetings from go-sandbox.\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
