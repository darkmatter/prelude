package motd

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
	"prelude/pkg/ui"
)

func TestCommandNoteRendersBesideCommandsLabel(t *testing.T) {
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
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "commands") {
			if !strings.Contains(line, "prefix with x if a command is shadowed") {
				t.Errorf("command note is not on the commands label row: %q", line)
			}
			return
		}
	}
	t.Errorf("MOTD output does not contain the commands label: %q", output)
}

func TestCommandNoteProseUsesUniformDimTone(t *testing.T) {
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
	s := sections{r: newRenderer(Resolve(cfg, Cache{}, 80, 20, time.Now()))}
	note := s.commandNote("prefix with `x` if a command is shadowed")
	dim := ui.Inline(s.r.st.dim)
	if !strings.Contains(note, dim.Render("prefix with ")) {
		t.Errorf("leading prose is not dim-styled: %q", note)
	}
	if !strings.Contains(note, dim.Render("if a command is shadowed")) {
		t.Errorf("trailing prose does not match the leading prose tone: %q", note)
	}
}

func TestCommandNoteFallsBackBelowListWhenNarrow(t *testing.T) {
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
		Width: 40,
	}

	output := ansi.Strip((MOTDView{r: newRenderer(Resolve(cfg, Cache{}, 40, 20, time.Now()))}).Render())
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "commands") && strings.Contains(line, "prefix with") {
			t.Errorf("note should drop below the list when the row is too narrow: %q", line)
		}
	}
	if !strings.Contains(output, "prefix with x if a command is shadowed") {
		t.Errorf("MOTD output lost the command note at narrow width: %q", output)
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
