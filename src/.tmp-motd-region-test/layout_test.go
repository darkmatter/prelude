package main

import (
	"strings"
	"testing"

	"prelude/shared"

	"charm.land/lipgloss/v2"
)

func TestPlaceUsesSingleWindowBackgroundAcrossFullWidth(t *testing.T) {
	cfg := Config{
		Palette: shared.Palette{
			Bg: shared.Color("#0e0d11"),
			Fg: shared.Color("#d6d2df"),
		},
		Background:         "#08070d",
		WindowBackground:   "#08070d",
		TerminalBackground: "#07070c",
		Align:              "center",
		Width:              10,
	}
	r := newRenderer(cfg, 20, nil)
	got := r.place(r.st.blockFill.Width(r.cardWidth).Render(""))

	if strings.Contains(got, "48;2;7;7;12") {
		t.Fatalf("placed line still paints terminal-colored gradient wings: %q", got)
	}
	if width := lipgloss.Width(got); width != 20 {
		t.Fatalf("placed width = %d, want full terminal width 20", width)
	}
}
