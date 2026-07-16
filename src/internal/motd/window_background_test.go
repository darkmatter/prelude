package motd

import (
	"strings"
	"testing"

	"prelude/pkg/shared"
)

func TestWindowBackgroundPaintsScreenErase(t *testing.T) {
	cfg := Config{
		Project:          "background-test",
		WindowBackground: "#112233",
		Background:       "#112233",
		ClearScreen:      true,
		Palette: shared.Palette{
			Fg:           "#ffffff",
			Muted:        "#aaaaaa",
			Dim:          "#777777",
			Border:       "#555555",
			AccentBorder: "#666666",
			Accent:       "#00aaff",
			Accent2:      "#ffaa00",
			Error:        "#ff0000",
			SelectionFg:  "#000000",
			Bg:           "#112233",
			Surface:      "#223344",
			Secondary:    "#334455",
		},
	}

	got := (MOTDView{r: newRenderer(cfg, 40, 24, systemRuntime{})}).Render()
	// Total newlines must be terminalHeight-1 (23) so the body ends on the
	// second-to-last row and the prompt lands on the bottom row without
	// scrolling.
	newlines := strings.Count(got, "\n")
	if newlines != 23 {
		t.Fatalf("newline count = %d, want 23 (terminalHeight-1, no scroll): output has %d bytes", newlines, len(got))
	}
	const clearScreen = "\x1b[2J\x1b[H"
	clearAt := strings.Index(got, clearScreen)
	if clearAt <= 0 {
		t.Fatalf("screen erase starts at byte %d; want a background SGR before it: %q", clearAt, got)
	}
	backgroundAt := strings.LastIndex(got[:clearAt], "\x1b[48;")
	if backgroundAt < 0 {
		t.Fatalf("screen erase is not painted with windowBackground: %q", got[:clearAt+len(clearScreen)])
	}

	// Fill-above rows must come AFTER the screen erase and BEFORE the MOTD body.
	fillStart := strings.Index(got[clearAt+len(clearScreen):], "\x1b[48;")
	if fillStart < 0 {
		t.Fatalf("no painted fill rows after screen erase")
	}

	// No ESC[H after the screen erase — the fill pushes the MOTD down so
	// the prompt lands under it without absolute homing.
	tail := got[clearAt+len(clearScreen):]
	if strings.Contains(tail, "\x1b[H") {
		t.Fatalf("fill-above contains absolute home (ESC[H): %q", tail)
	}

	// No cursor-up (ESC[nA) — fill is above the body, not below.
	if strings.Contains(tail, "\x1b[") && strings.Contains(tail, "A") {
		// Check for actual cursor-up sequence ESC[<digits>A
		for i := 0; i < len(tail)-3; i++ {
			if tail[i] == '\x1b' && tail[i+1] == '[' {
				j := i + 2
				for j < len(tail) && tail[j] >= '0' && tail[j] <= '9' {
					j++
				}
				if j < len(tail) && tail[j] == 'A' {
					t.Fatalf("fill-above should not emit cursor-up: %q", tail[i:j+1])
				}
			}
		}
	}
}

func TestWindowBackgroundTransparentSkipsScrollFill(t *testing.T) {
	cfg := Config{
		Project:     "transparent-test",
		ClearScreen: true,
		Palette: shared.Palette{
			Fg:           "#ffffff",
			Muted:        "#aaaaaa",
			Dim:          "#777777",
			Border:       "#555555",
			AccentBorder: "#666666",
			Accent:       "#00aaff",
			Accent2:      "#ffaa00",
			Error:        "#ff0000",
			SelectionFg:  "#000000",
			Bg:           "#112233",
			Surface:      "#223344",
			Secondary:    "#334455",
		},
	}

	got := (MOTDView{r: newRenderer(cfg, 40, 24, systemRuntime{})}).Render()
	const clearScreen = "\x1b[2J\x1b[H"
	// Transparent window: clear screen must NOT have a background SGR.
	clearAt := strings.Index(got, clearScreen)
	if clearAt != 0 {
		t.Fatalf("transparent clear should be first. got prefix: %q", got[:clearAt])
	}
	// No fill rows when window is transparent.
	if strings.Contains(got[len(clearScreen):], "\x1b[48;") {
		t.Fatalf("transparent window should not emit fill rows")
	}
}
