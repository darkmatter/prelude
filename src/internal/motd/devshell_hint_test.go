package motd

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const devshellHintCopy = "You're using the cached devshell, which doesn't run the shellHook in your interactive shell. Run nix develop for the full devshell."

func TestRenderDevshellHintUsesCardGeometry(t *testing.T) {
	cases := []struct {
		name       string
		width      int
		align      string
		border     bool
		wantCard   int
		wantOffset int
	}{
		{name: "narrow left", width: 30, align: "left", wantCard: 23, wantOffset: 4},
		{name: "wide centered bordered", width: 120, align: "center", border: true, wantCard: 64, wantOffset: 28},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := opaqueCardConfig()
			cfg.Description.Text = "CARD"
			cfg.Width = 0
			cfg.MaxWidth = 64
			cfg.Align = tc.align
			cfg.Border = tc.border
			cfg.Margin = Spacing{Top: 1, Bottom: 1, Left: 4, Right: 3}
			cfg.Padding = Spacing{Left: 2, Right: 2}
			model := Resolve(cfg, Cache{}, tc.width, 40, time.Now())
			if model.CardWidth != tc.wantCard || model.HorizontalOffset != tc.wantOffset {
				t.Fatalf("resolved card geometry = (%d, %d), want (%d, %d)", model.CardWidth, model.HorizontalOffset, tc.wantCard, tc.wantOffset)
			}

			input := RenderInput{Config: cfg, Cache: Cache{}, TerminalWidth: tc.width, TerminalHeight: 40}
			cardRows := strings.Split(ansi.Strip(Render(input)), "\n")
			// Exclude the window's bottom margin and trailing newline, not the
			// card's own painted blank rows.
			lastCard := len(cardRows) - max(model.Margin.Bottom, 0) - 2
			if lastCard < 0 {
				t.Fatal("baseline card is empty")
			}
			if tc.border && !strings.Contains(cardRows[lastCard], "╰") {
				t.Fatalf("last card row is not the bottom border: %q", cardRows[lastCard])
			}

			input.DevshellHint = true
			output := Render(input)
			rows := strings.Split(ansi.Strip(output), "\n")
			firstHint, hintRows := renderedDevshellHintRows(t, output)
			if got := strings.Join(rows[:lastCard+1], "\n"); got != strings.Join(cardRows[:lastCard+1], "\n") {
				t.Fatalf("notice changed the card or its placement:\n%s", ansi.Strip(output))
			}
			if got := firstHint - lastCard - 1; got != 4 {
				t.Fatalf("blank rows after the last card row = %d, want exactly 4:\n%s", got, ansi.Strip(output))
			}
			for row := lastCard + 1; row < firstHint; row++ {
				if strings.TrimSpace(rows[row]) != "" {
					t.Fatalf("gap row %d is not blank: %q", row, rows[row])
				}
			}

			var text []string
			for index, row := range hintRows {
				line := ansi.Strip(row)
				content := strings.TrimSpace(line)
				width := lipgloss.Width(content)
				leading := lipgloss.Width(line) - lipgloss.Width(strings.TrimLeft(line, " "))
				left := leading - model.HorizontalOffset
				right := model.CardWidth - left - width
				if width > model.ContentWidth || left < cfg.Padding.Left || right < cfg.Padding.Right {
					t.Fatalf("notice row %d exceeds the card: text width=%d, left=%d, right=%d, card=%d: %q", index, width, left, right, model.CardWidth, line)
				}
				if difference := left - right; difference < -1 || difference > 1 {
					t.Fatalf("notice row %d is not centered within the card: left=%d, right=%d: %q", index, left, right, line)
				}
				if lipgloss.Width(row) > tc.width {
					t.Fatalf("notice row %d exceeds the terminal width: %q", index, line)
				}
				text = append(text, content)
			}
			if got := strings.Join(strings.Fields(strings.Join(text, " ")), " "); got != devshellHintCopy {
				t.Fatalf("wrapped notice lost or changed text:\ngot  %q\nwant %q", got, devshellHintCopy)
			}
		})
	}
}

func TestRenderDevshellHintStylesOnlyTheCommand(t *testing.T) {
	cfg := opaqueCardConfig()
	cfg.Description.Text = "CARD"
	cfg.Width = 160
	cfg.Border = true
	cfg.Align = "center"
	output := Render(RenderInput{Config: cfg, Cache: Cache{}, TerminalWidth: 200, TerminalHeight: 40, DevshellHint: true})
	_, rows := renderedDevshellHintRows(t, output)
	if len(rows) != 1 {
		t.Fatalf("wide notice has %d rows, want one", len(rows))
	}

	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Palette.Muted.String())).Inline(true)
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(cfg.Palette.Accent.String())).Inline(true)
	before, after, found := strings.Cut(devshellHintCopy, "nix develop")
	if !found {
		t.Fatal("notice copy does not contain the command")
	}
	want := muted.Render(before) + accent.Render("nix develop") + muted.Render(after)
	if !strings.Contains(rows[0], want) {
		t.Fatalf("notice is not regular muted prose with only the command accented:\ngot  %q\nwant %q", rows[0], want)
	}
	// The fixture's foreground RGB components contain no 1 or 48, so these
	// checks distinguish bold/background attributes from color components.
	for _, bold := range []string{"\x1b[1m", "\x1b[1;", ";1m", ";1;"} {
		if strings.Contains(rows[0], bold) {
			t.Fatalf("notice contains bold styling: %q", rows[0])
		}
	}
	if !strings.Contains(output, "48;2;17;34;51") {
		t.Fatal("fixture did not render an opaque card")
	}
	if strings.Contains(rows[0], "48;") {
		t.Fatalf("notice inherited a non-default background: %q", rows[0])
	}
}

func TestRenderDevshellHintIsOptIn(t *testing.T) {
	cfg := opaqueCardConfig()
	cfg.Description.Text = "CARD"
	input := RenderInput{Config: cfg, Cache: Cache{}, TerminalWidth: 80, TerminalHeight: 24}
	withoutHint := Render(input)
	plain := ansi.Strip(withoutHint)
	if !strings.Contains(plain, "CARD") {
		t.Fatalf("default Render lost the card: %q", plain)
	}
	if strings.Contains(plain, "You're") || strings.Contains(plain, "nix develop") {
		t.Fatalf("default Render included a loader notice: %q", plain)
	}

	input.DevshellHint = true
	renderedDevshellHintRows(t, Render(input))
	input.DevshellHint = false
	if got := Render(input); got != withoutHint {
		t.Fatalf("explicitly disabled notice changed the default render:\ngot  %q\nwant %q", got, withoutHint)
	}
}

func renderedDevshellHintRows(t *testing.T, output string) (int, []string) {
	t.Helper()
	rows := strings.Split(output, "\n")
	for first, row := range rows {
		if !strings.HasPrefix(strings.TrimSpace(ansi.Strip(row)), "You're") {
			continue
		}
		last := len(rows)
		for last > first && strings.TrimSpace(ansi.Strip(rows[last-1])) == "" {
			last--
		}
		return first, rows[first:last]
	}
	t.Fatalf("render has no cached-devshell notice:\n%s", ansi.Strip(output))
	return 0, nil
}
