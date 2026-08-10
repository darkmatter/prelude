package motd

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
)

func TestOpaqueCardRendersWithoutFringeAndReclaimsWidth(t *testing.T) {
	cfg := opaqueCardConfig()
	cfg.Width = 0
	model := Resolve(cfg, Cache{}, 20, 24, time.Now())

	if model.CardWidth != 20 {
		t.Fatalf("card width = %d, want all 20 columns with zero configured margins", model.CardWidth)
	}
	if model.HorizontalOffset != 0 {
		t.Fatalf("horizontal offset = %d, want zero with zero configured left margin", model.HorizontalOffset)
	}

	output := render(model)
	if strings.ContainsAny(output, "░▒▓") {
		t.Fatalf("opaque card rendered obsolete fringe glyphs: %q", ansi.Strip(output))
	}
	if !strings.Contains(ansi.Strip(output), "x") {
		t.Fatalf("card content missing from render: %q", ansi.Strip(output))
	}
}

func TestOpaqueCardResetsAnInheritedBackground(t *testing.T) {
	output := "\x1b[48;2;1;2;3m" + render(Resolve(opaqueCardConfig(), Cache{}, 20, 24, time.Now()))
	if !strings.Contains(output, "\x1b[49m") {
		t.Fatal("render did not reset the inherited terminal background")
	}
	if !strings.HasSuffix(output, "\x1b[49m") {
		t.Fatalf("render did not return the prompt to the terminal-default background: %q", output[len(output)-min(len(output), 32):])
	}
}

func TestResponsiveCardGeometryUsesAvailableWidth(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		wantCard  int
		wantEmpty bool
	}{
		{name: "wide", width: 20, wantCard: 20},
		{name: "medium", width: 17, wantCard: 17},
		{name: "minimum", width: 10, wantCard: 10},
		{name: "impossible width", width: 9, wantEmpty: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := opaqueCardConfig()
			cfg.Width = 0
			model := Resolve(cfg, Cache{}, tc.width, 24, time.Now())
			output := render(model)
			if tc.wantEmpty {
				if output != "" {
					t.Fatalf("narrow terminal rendered output instead of omitting MOTD: %q", ansi.Strip(output))
				}
				return
			}
			if model.CardWidth != tc.wantCard {
				t.Fatalf("card width = %d, want %d", model.CardWidth, tc.wantCard)
			}
			if model.HorizontalOffset != 0 {
				t.Fatalf("horizontal offset = %d, want zero with zero configured left margin", model.HorizontalOffset)
			}
			if strings.ContainsAny(output, "░▒▓") {
				t.Fatalf("render contains obsolete fringe glyphs: %q", ansi.Strip(output))
			}
		})
	}
}

func TestResponsiveGeometryAlignsTheCard(t *testing.T) {
	cfg := opaqueCardConfig()
	cfg.Width = 100
	cfg.MaxWidth = 40
	cfg.Align = "center"
	cfg.Margin = Spacing{Left: 4, Right: 3}

	model := Resolve(cfg, Cache{}, 80, 24, time.Now())
	if model.CardWidth != 40 {
		t.Fatalf("card width = %d, want maxWidth 40", model.CardWidth)
	}
	// available = 80 - 4 - 3 = 73. Center the 40-cell card directly:
	// 4 + floor((73-40)/2) = 20.
	if model.HorizontalOffset != 20 {
		t.Fatalf("horizontal offset = %d, want 20", model.HorizontalOffset)
	}
}

func TestClearScreenUsesTransparentNewlinePositioning(t *testing.T) {
	const terminalHeight = 24
	const clearPrefix = "\x1b[49m\x1b[2J\x1b[H\x1b[49m"

	base := opaqueCardConfig()
	plainOutput := render(Resolve(base, Cache{}, 20, terminalHeight, time.Now()))
	bodyRows := strings.Count(plainOutput, "\n")
	fillRows := terminalHeight - bodyRows - 1
	baselineCardRow := firstStyledRow(plainOutput, "\x1b[48;2;17;34;51m")
	tests := []struct {
		align     string
		wantAbove int
	}{
		{align: "top", wantAbove: 0},
		{align: "center", wantAbove: fillRows / 2},
		{align: "bottom", wantAbove: fillRows},
	}
	for _, tc := range tests {
		t.Run(tc.align, func(t *testing.T) {
			cfg := base
			cfg.ClearScreen = true
			cfg.VerticalAlign = tc.align
			model := Resolve(cfg, Cache{}, 20, terminalHeight, time.Now())
			output := render(model)

			if !strings.HasPrefix(output, clearPrefix) {
				t.Fatalf("clear-screen prefix = %q, want explicit default background around ED2", output[:min(len(output), len(clearPrefix)+24)])
			}
			if !strings.HasSuffix(output, "\x1b[49m") {
				t.Fatal("clear-screen render did not reset the prompt background")
			}
			if got := strings.Count(output, "\n"); got != terminalHeight-1 {
				t.Fatalf("newline count = %d, want %d", got, terminalHeight-1)
			}
			firstCard := firstStyledRow(output, "\x1b[48;2;17;34;51m")
			wantFirstCard := tc.wantAbove + baselineCardRow
			if firstCard != wantFirstCard {
				t.Fatalf("first painted card row = %d, want %d", firstCard, wantFirstCard)
			}

			wantPositioning := clearPrefix + strings.Repeat("\n", tc.wantAbove) + strings.Repeat(" ", model.HorizontalOffset)
			if !strings.HasPrefix(output, wantPositioning) {
				t.Fatalf("clear-screen render does not begin with newline-only rows and the horizontal inset: %q", ansi.Strip(output[:min(len(output), len(wantPositioning)+24)]))
			}
		})
	}
}

func TestTransparentCardRemainsUnpainted(t *testing.T) {
	cfg := opaqueCardConfig()
	cfg.Background = ""

	output := render(Resolve(cfg, Cache{}, 20, 24, time.Now()))
	if strings.ContainsAny(output, "░▒▓") {
		t.Fatalf("transparent card rendered obsolete fringe glyphs: %q", ansi.Strip(output))
	}
	if strings.Contains(output, "\x1b[48;") {
		t.Fatalf("transparent card painted a non-default background: %q", output)
	}
	if !strings.HasSuffix(output, "\x1b[49m") {
		t.Fatal("transparent render did not hand the terminal-default background back to the prompt")
	}
}

func opaqueCardConfig() Config {
	return Config{
		Background:  "#112233",
		Description: StyledText{Text: "x"},
		Width:       10,
		Palette: shared.Palette{
			Fg:           "#ffffff",
			Muted:        "#aaaaaa",
			Dim:          "#777777",
			Border:       "#555555",
			AccentBorder: "#666666",
			Accent:       "#00aaff",
			Accent2:      "#ffaa00",
			Success:      "#00ff00",
			Warning:      "#ffaa00",
			Info:         "#00aaff",
			Error:        "#ff0000",
			SelectionFg:  "#000000",
			Bg:           "#112233",
			Surface:      "#223344",
			Secondary:    "#334455",
		},
	}
}

func firstStyledRow(output, style string) int {
	for row, line := range strings.Split(output, "\n") {
		if strings.Contains(line, style) {
			return row
		}
	}
	return -1
}
