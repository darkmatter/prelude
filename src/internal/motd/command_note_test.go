package motd

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
)

func TestCommandNoteRendersUnderCommandsList(t *testing.T) {
	cfg := Config{
		Palette: shared.Palette{
			Bg:     "#101010",
			Accent: "#00aaff",
		},
		Commands: []Command{
			{Command: "test", Description: "run the tests"},
		},
		GettingStarted: GettingStarted{
			CommandNote: "prefix with `x` if a command is shadowed",
		},
		Width: 80,
	}

	output := ansi.Strip((MOTDView{r: newRenderer(Resolve(cfg, Cache{}, 80, 20, time.Now()))}).Render())
	if !strings.Contains(output, "prefix with x if a command is shadowed") {
		t.Errorf("MOTD output does not contain the command note: %q", output)
	}
}

func TestEmptyCommandNoteIsHidden(t *testing.T) {
	cfg := Config{
		Palette: shared.Palette{
			Bg:     "#101010",
			Accent: "#00aaff",
		},
		Commands: []Command{
			{Command: "test", Description: "run the tests"},
		},
		Width: 80,
	}

	output := (MOTDView{r: newRenderer(Resolve(cfg, Cache{}, 80, 20, time.Now()))}).Render()
	if !strings.Contains(output, "test") {
		t.Errorf("MOTD output does not contain the command row: %q", output)
	}
	if strings.Contains(output, "shadowed") {
		t.Errorf("MOTD output renders a note without a configured command note: %q", output)
	}
}
