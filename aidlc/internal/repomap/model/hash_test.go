package model

import (
	"strings"
	"testing"
)

func TestContentHash(t *testing.T) {
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := ContentHash([]byte("abc")); got != want {
		t.Fatalf("ContentHash() = %q, want %q", got, want)
	}
}

func TestContentHashReader(t *testing.T) {
	const want = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	got, err := ContentHashReader(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ContentHashReader() error = %v", err)
	}
	if got != want {
		t.Fatalf("ContentHashReader() = %q, want %q", got, want)
	}
}
