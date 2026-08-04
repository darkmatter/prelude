package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSanitizePinKeepsSGRDropsCursor(t *testing.T) {
	in := "\x1b[1;1H\x1b[2J\x1b[38;5;81mhello\x1b[0m\x1b[2;1Hworld"
	got := sanitizePinANSI(in)
	if !strings.Contains(got, "\x1b[38;5;81mhello\x1b[0m") {
		t.Fatalf("expected SGR kept, got %q", got)
	}
	if strings.Contains(got, "\x1b[1;1H") || strings.Contains(got, "\x1b[2J") || strings.Contains(got, "\x1b[2;1H") {
		t.Fatalf("cursor/erase should be dropped: %q", got)
	}
	// printable text retained in order
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "hello") || !strings.Contains(plain, "world") {
		t.Fatalf("text missing: %q", plain)
	}
}

func TestPadTruncatePreservesSGR(t *testing.T) {
	s := "\x1b[31mred\x1b[0m"
	got := padTruncate(s, 10)
	if !strings.Contains(got, "\x1b[31mred\x1b[0m") {
		t.Fatalf("SGR lost: %q", got)
	}
	if ansi.StringWidth(got) != 10 {
		t.Fatalf("width want 10 got %d (%q)", ansi.StringWidth(got), got)
	}
}

func TestPadTruncateCutsLongColored(t *testing.T) {
	s := "\x1b[32m" + strings.Repeat("x", 50) + "\x1b[0m"
	got := padTruncate(s, 10)
	if ansi.StringWidth(got) != 10 {
		t.Fatalf("width want 10 got %d", ansi.StringWidth(got))
	}
	if !strings.Contains(got, "\x1b[32m") {
		t.Fatalf("expected color kept after truncate: %q", got)
	}
}
