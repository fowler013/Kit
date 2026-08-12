package main

import (
	"strings"
	"testing"
)

func TestSplitMessage(t *testing.T) {
	// short message stays intact
	if got := splitMessage("hello", 1900); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("short message mangled: %v", got)
	}
	// long message with newlines splits under limit and preserves content
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteString("• Camper Name L. (returning, paid)\n")
	}
	text := strings.TrimRight(b.String(), "\n")
	chunks := splitMessage(text, 1900)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len(c) > 1900 {
			t.Errorf("chunk %d exceeds limit: %d chars", i, len(c))
		}
	}
	rejoined := strings.Join(chunks, "\n")
	if rejoined != text {
		t.Errorf("content lost after split/rejoin")
	}
	// pathological: no newlines at all
	long := strings.Repeat("x", 5000)
	for i, c := range splitMessage(long, 1900) {
		if len(c) > 1900 {
			t.Errorf("no-newline chunk %d exceeds limit: %d", i, len(c))
		}
	}
}
