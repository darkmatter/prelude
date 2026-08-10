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

// TestCommandNoteGapPaintedWithCardBackground verifies that the gap between
// the commands label and note carries the same explicit fill as the card.
// Styled segments reset SGR, so a wrapper-level background is insufficient.
func TestCommandNoteGapPaintedWithCardBackground(t *testing.T) {
	cfg := Config{
		Palette: shared.Palette{
			Bg:     "#101010",
			Fg:     "#ffffff",
			Dim:    "#777777",
			Accent: "#00aaff",
		},
		Background: "#112233",
		Commands: []Command{
			{Command: "test", Description: "run the tests"},
		},
		GettingStarted: GettingStarted{
			CommandNote: "prefix with `x` if a command is shadowed",
		},
		Width: 80,
	}

	output := (MOTDView{r: newRenderer(Resolve(cfg, Cache{}, 80, 20, time.Now()))}).Render()

	// Find the commands header row containing both the label and the note.
	for _, line := range strings.Split(output, "\n") {
		stripped := ansi.Strip(line)
		if !strings.Contains(stripped, "commands") || !strings.Contains(stripped, "prefix with") {
			continue
		}
		// #112233 → 48;2;17;34;51 in truecolor SGR.
		if !strings.Contains(line, "\x1b[48;2;17;34;51m") {
			t.Errorf("gap is not painted with the card background;\nrow: %q", line)
		}
		return
	}
	t.Fatalf("commands header row not found in output")
}
