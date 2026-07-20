package motd

import (
	"strings"
	"testing"

	"prelude/pkg/shared"
)

func TestConfiguredShortcutsRenderWithoutGeneratedTitle(t *testing.T) {
	cfg := Config{
		Palette: shared.Palette{
			Bg:     "#101010",
			Accent: "#00aaff",
		},
		Header: Header{Tagline: "Dev Shell Activated"},
		Shortcuts: []Shortcut{
			{Command: "motd", Alias: "?"},
			{Command: "menu", Alias: "m"},
			{Command: "docs", Alias: "d"},
		},
		Width: 80,
	}

	output := (MOTDView{r: newRenderer(cfg, 80, 20)}).Render()
	for _, want := range []string{"motd", "menu", "docs"} {
		if count := strings.Count(output, want); count != 1 {
			t.Errorf("MOTD output contains shortcut %q %d times, want once: %q", want, count, output)
		}
	}
	for _, want := range []string{"?", "m", "d"} {
		if !strings.Contains(output, want) {
			t.Errorf("MOTD output does not contain configured shortcut alias %q: %q", want, output)
		}
	}
}
