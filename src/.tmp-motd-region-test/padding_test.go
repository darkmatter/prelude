package main

import (
	"strings"
	"testing"

	"prelude/shared"

	"github.com/charmbracelet/x/ansi"
)

func TestTopPaddingAppearsBeforeTitle(t *testing.T) {
	cfg := Config{
		Project:          "demo",
		Palette:          testPalette(),
		Background:       "#101010",
		WindowBackground: "#101010",
		Padding:          Spacing{Top: 2},
		Header:           Header{TitleStyle: titleStylePlain},
		Width:            24,
	}

	body := newRenderer(cfg, 24, nil).renderBody()
	lines := strings.Split(body, "\n")
	if len(lines) < 3 {
		t.Fatalf("rendered only %d lines", len(lines))
	}
	for i := 0; i < 2; i++ {
		if strings.TrimSpace(ansi.Strip(lines[i])) != "" {
			t.Fatalf("line %d = %q, want top padding before title", i, ansi.Strip(lines[i]))
		}
	}
	if !strings.Contains(ansi.Strip(lines[2]), "demo") {
		t.Fatalf("line 2 = %q, want title after top padding", ansi.Strip(lines[2]))
	}
}

func testPalette() shared.Palette {
	return shared.Palette{
		Fg:           "#dddddd",
		Muted:        "#999999",
		Dim:          "#666666",
		Border:       "#555555",
		AccentBorder: "#777777",
		Accent:       "#ff00aa",
		Accent2:      "#00ffaa",
		Error:        "#ff5555",
		SelectionFg:  "#101010",
		Bg:           "#101010",
		Surface:      "#181818",
		Secondary:    "#202020",
	}
}
