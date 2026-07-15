package main

import (
	"testing"

	"prelude/shared"

	"charm.land/lipgloss/v2"
)

func TestResolveWindowBackgroundBlend(t *testing.T) {
	terminal := lipgloss.Color("#202830")
	cfg := Config{
		Palette:                  shared.Palette{Bg: shared.Color("#604080")},
		WindowBackgroundBlend:    0.15,
		WindowBackgroundBlendSet: true,
	}

	got := resolveRelativeBackgrounds(cfg, terminal)
	want := colorHex(lipgloss.Blend1D(101, terminal, lipgloss.Color("#604080"))[15])

	if got.WindowBackground != want {
		t.Fatalf("window background = %s, want 15%% blend %s", got.WindowBackground, want)
	}
	if got.TerminalBackground != "#202830" {
		t.Fatalf("terminal background = %s, want original query color", got.TerminalBackground)
	}
}

func TestResolveZeroWindowBackgroundBlend(t *testing.T) {
	terminal := lipgloss.Color("#202830")
	cfg := Config{
		Palette:                  shared.Palette{Bg: shared.Color("#604080")},
		WindowBackgroundBlend:    0,
		WindowBackgroundBlendSet: true,
	}

	got := resolveRelativeBackgrounds(cfg, terminal)
	if got.WindowBackground != "#202830" {
		t.Fatalf("zero blend background = %s, want terminal color", got.WindowBackground)
	}
}

func TestResolveCardBackgroundBlend(t *testing.T) {
	terminal := lipgloss.Color("#202830")
	cfg := Config{
		Palette:            shared.Palette{Bg: shared.Color("#604080")},
		BackgroundBlend:    0.15,
		BackgroundBlendSet: true,
	}

	got := resolveRelativeBackgrounds(cfg, terminal)
	want := colorHex(lipgloss.Blend1D(101, terminal, lipgloss.Color("#604080"))[15])

	if got.Background != want {
		t.Fatalf("card background = %s, want 15%% blend %s", got.Background, want)
	}
}
