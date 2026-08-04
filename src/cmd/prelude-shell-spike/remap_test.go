package main

import (
	"bytes"
	"testing"
)

func TestRemapCUPOffsetsIntoShellStrip(t *testing.T) {
	// Pin occupies rows 1..20; shell is rows 21..30 (shellTop=21, shellH=10).
	in := []byte("\x1b[1;1H\x1b[2;5Hhi")
	got := remapShellOutput(in, 21, 10, 80)
	// row 1 → 21, row 2 → 22
	if !bytes.Contains(got, []byte("\x1b[21;1H")) {
		t.Fatalf("missing offset CUP home: %q", got)
	}
	if !bytes.Contains(got, []byte("\x1b[22;5H")) {
		t.Fatalf("missing offset CUP 2;5: %q", got)
	}
	if bytes.Contains(got, []byte("\x1b[1;1H")) {
		t.Fatalf("raw CUP 1;1 leaked into host: %q", got)
	}
}

func TestRemapDropsChildScrollRegion(t *testing.T) {
	in := []byte("\x1b[1;24r")
	got := remapShellOutput(in, 5, 10, 80)
	if len(got) != 0 {
		t.Fatalf("expected child DECSTBM dropped, got %q", got)
	}
}

func TestRemapClearDoesNotUseFullScreenED(t *testing.T) {
	in := []byte("\x1b[2J")
	got := remapShellOutput(in, 5, 3, 10)
	if bytes.Contains(got, []byte("\x1b[2J")) {
		t.Fatalf("full ED should be replaced: %q", got)
	}
	// Should clear shell rows 5,6,7
	if !bytes.Contains(got, []byte("\x1b[5;1H")) || !bytes.Contains(got, []byte("\x1b[7;1H")) {
		t.Fatalf("expected shell-region clear, got %q", got)
	}
}

func TestRemapDropsOriginModeAndAltScreen(t *testing.T) {
	cases := []struct {
		in   string
		want string // empty => fully dropped
	}{
		{"\x1b[?6h", ""},
		{"\x1b[?6l", ""},
		{"\x1b[?1049h", ""},
		{"\x1b[?47h", ""},
		{"\x1b[?1;6h", "\x1b[?1h"}, // keep app-cursor, drop DECOM
		{"\x1b[?1h", "\x1b[?1h"},
	}
	for _, tc := range cases {
		got := string(remapShellOutput([]byte(tc.in), 5, 10, 80))
		if got != tc.want {
			t.Fatalf("in %q: got %q want %q", tc.in, got, tc.want)
		}
	}
}
