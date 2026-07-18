package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// A transparent glow rule must not paint cell backgrounds (SGR 48): the
// gradient blends toward the context background, but the cells themselves
// stay on whatever background the terminal shows.
func TestGlowRuleTransparencySkipsCellBackgrounds(t *testing.T) {
	background := lipgloss.Color("#102030")
	peak := lipgloss.Color("#ff97d7")

	transparent := GlowRule{
		Context: NewContext(Palette{}, background, true),
		Width:   8,
		Glyph:   "━",
		Peak:    peak,
	}.Render()
	if strings.Contains(transparent, ";48;2;") {
		t.Fatalf("transparent rule paints backgrounds: %q", transparent)
	}
	if !strings.Contains(transparent, "[38;2;") {
		t.Fatalf("transparent rule lost its foreground gradient: %q", transparent)
	}

	opaque := GlowRule{
		Context: NewContext(Palette{}, background, false),
		Width:   8,
		Glyph:   "━",
		Peak:    peak,
	}.Render()
	if !strings.Contains(opaque, ";48;2;") {
		t.Fatalf("opaque rule stopped painting backgrounds: %q", opaque)
	}
}
