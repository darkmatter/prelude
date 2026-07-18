package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestLinkRendersOSC8Hyperlink(t *testing.T) {
	const (
		label = "github.com/darkmatter/prelude"
		url   = "https://github.com/darkmatter/prelude"
	)
	link := Link{
		Context: NewContext(Palette{Accent: "#00aaff"}, nil, true),
		Label:   label,
		URL:     url,
	}

	got := link.Render()
	if !strings.Contains(got, "\x1b]8;") {
		t.Fatalf("rendered link has no OSC 8 sequence: %q", got)
	}
	if !strings.Contains(got, url) {
		t.Fatalf("rendered link has no target URL: %q", got)
	}
	if plain := ansi.Strip(got); plain != label {
		t.Fatalf("visible link = %q, want %q", plain, label)
	}
	if width := lipgloss.Width(got); width != lipgloss.Width(label) {
		t.Fatalf("rendered width = %d, want %d", width, lipgloss.Width(label))
	}
}
