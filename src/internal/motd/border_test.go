package motd

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
)

func TestMOTDBorderToggle(t *testing.T) {
	base := Config{
		Title:      "AA\nAAAA",
		TitleAlign: "left",
		Width:      20,
		Palette: shared.Palette{
			Fg:           "#ffffff",
			Muted:        "#aaaaaa",
			Dim:          "#777777",
			Border:       "#555555",
			AccentBorder: "#666666",
			Accent:       "#00aaff",
			Bg:           "#112233",
			Surface:      "#223344",
		},
	}

	withoutBorder := render(Resolve(base, Cache{}, 20, 24, time.Now()))
	if strings.Contains(withoutBorder, "╭") || strings.Contains(withoutBorder, "╰") {
		t.Fatalf("border=false rendered a frame:\n%s", withoutBorder)
	}

	base.Border = true
	withBorder := render(Resolve(base, Cache{}, 20, 24, time.Now()))
	if !strings.Contains(withBorder, "╭") || !strings.Contains(withBorder, "╰") {
		t.Fatalf("border=true did not render a frame:\n%s", withBorder)
	}
	for index, line := range strings.Split(withBorder, "\n") {
		if got := lipgloss.Width(line); got > 20 {
			t.Fatalf("bordered row %d width = %d, want <= 20: %q", index, got, line)
		}
	}
}
