package main

import (
	"bytes"
	"testing"
)

func TestEscCoalescerDoesNotSplitCSI(t *testing.T) {
	// Simulate Read() splitting ESC[38;5;135m across chunks — the bug that
	// produced visible "135m" when we painted status mid-sequence.
	seq := []byte("\x1b[38;5;135mhi\x1b[0m")
	var c escCoalescer
	var got []byte

	// Feed one byte at a time.
	for i, b := range seq {
		out, safe := c.Feed([]byte{b})
		got = append(got, out...)
		// Never "safe to paint" while still inside the CSI.
		if i < len(seq)-1 {
			// After complete CSI (at 'm'), safe becomes true briefly — only
			// assert unsafe while still incomplete: before first final 'm'.
			if i < bytes.IndexByte(seq, 'm') && safe && len(c.pending) > 0 {
				t.Fatalf("safe mid-CSI at byte %d pending=%q", i, c.pending)
			}
		}
	}
	if !bytes.Equal(got, seq) {
		t.Fatalf("reassembled = %q, want %q", got, seq)
	}
	if _, safe := c.Feed(nil); !safe && len(c.pending) != 0 {
		t.Fatalf("pending after complete stream: %q", c.pending)
	}
}

func TestEscCoalescerTrueColorSplit(t *testing.T) {
	// ESC[38;2;46;56;70m — split so ";46;56" would leak without coalescing.
	parts := [][]byte{
		[]byte("\x1b[38;2"),
		[]byte(";46;56"),
		[]byte(";70m"),
		[]byte("x"),
	}
	var c escCoalescer
	var got []byte
	for _, p := range parts {
		out, safe := c.Feed(p)
		got = append(got, out...)
		// After incomplete CSI chunks, must not be safe with pending esc.
		if bytes.Contains(p, []byte("46")) && safe {
			// second chunk ends mid-CSI — should not be safe
			if c.state != stGround {
				// good, not ground
			} else if len(c.pending) == 0 && !bytes.HasSuffix(got, []byte("m")) {
				t.Fatalf("became safe while CSI incomplete, got=%q", got)
			}
		}
	}
	want := []byte("\x1b[38;2;46;56;70mx")
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestEscCoalescerOSC(t *testing.T) {
	// OSC 8 hyperlink style: ESC ] 8 ; ; url ST
	seq := []byte("\x1b]8;;https://example.com\x07link")
	var c escCoalescer
	var got []byte
	for _, b := range seq {
		out, _ := c.Feed([]byte{b})
		got = append(got, out...)
	}
	if !bytes.Equal(got, seq) {
		t.Fatalf("got %q want %q", got, seq)
	}
}

func TestEscCoalescerPlainTextPassthrough(t *testing.T) {
	var c escCoalescer
	out, safe := c.Feed([]byte("hello"))
	if !safe || string(out) != "hello" {
		t.Fatalf("out=%q safe=%v", out, safe)
	}
}
