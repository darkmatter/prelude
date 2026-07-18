package motd

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
)

func TestStatusItemsInlineHintAlignsStatusAndHint(t *testing.T) {
	r := renderer{cfg: Config{StatusAge: "17m ago", StatusHint: "[r] to reload"}}
	items := []HeaderStatus{{Label: "dev server", Status: "ready"}}
	statusItems := StatusItems{r: r}
	status := statusItems.Render(items, false)
	hint := statusItems.Hint(false)
	width := lipgloss.Width(status) + lipgloss.Width(hint) + 7

	row := statusItems.InlineHint(items, width, false)

	if !strings.HasPrefix(row, status) {
		t.Fatalf("row does not start with status: %q", row)
	}
	if !strings.HasSuffix(row, hint) {
		t.Fatalf("row does not end with hint: %q", row)
	}
	if got := lipgloss.Width(row); got != width {
		t.Fatalf("row width = %d, want %d", got, width)
	}
}

func TestStatusItemsHintAppendsHyperlinksAfterReloadHint(t *testing.T) {
	r := renderer{cfg: Config{
		StatusHint: "[r] to reload",
		Header: Header{StatusHintLinks: []Link{
			{Label: "github", URL: "https://github.com/darkmatter/prelude"},
		}},
	}}
	hint := StatusItems{r: r}.Hint(false)

	plain := ansi.Strip(hint)
	reload := strings.Index(plain, "[r] to reload")
	label := strings.Index(plain, "github")
	if reload < 0 || label < 0 || label < reload {
		t.Fatalf("hint order wrong: %q", plain)
	}
	if !strings.Contains(plain, "  ·  ") {
		t.Fatalf("hint missing separator: %q", plain)
	}
	// The label must be an OSC 8 hyperlink carrying the URL, not visible text.
	if !strings.Contains(hint, "]8;;https://github.com/darkmatter/prelude") {
		t.Fatalf("hint missing OSC 8 target: %q", hint)
	}
}

func TestStatusItemsHintRendersLinksWithoutReloadHint(t *testing.T) {
	r := renderer{cfg: Config{
		Header: Header{StatusHintLinks: []Link{{Label: "github", URL: "https://example.com"}}},
	}}
	plain := ansi.Strip(StatusItems{r: r}.Hint(false))
	if !strings.Contains(plain, "github") || strings.Contains(plain, "·") {
		t.Fatalf("link-only hint wrong: %q", plain)
	}
}

func TestStatusItemsInlineHintCompactsBeforeFallingBack(t *testing.T) {
	r := renderer{cfg: Config{StatusHint: "[r] to reload"}}
	items := []HeaderStatus{{Label: "dev server", Status: "ready"}}
	statusItems := StatusItems{r: r}
	compact := statusItems.Render(items, true)
	hint := statusItems.Hint(false)
	width := lipgloss.Width(compact) + lipgloss.Width(hint) + 1

	row := statusItems.InlineHint(items, width, false)

	if !strings.HasPrefix(row, compact) {
		t.Fatalf("row did not use compact status: %q", row)
	}
	if got := statusItems.InlineHint(items, width-1, false); got != "" {
		t.Fatalf("too-narrow row = %q, want stacked-layout fallback", got)
	}
}

func TestStatusItemsUseSemanticStatusColors(t *testing.T) {
	palette := shared.Palette{
		Success: "#11aa22",
		Warning: "#ddaa00",
		Info:    "#2277cc",
		Error:   "#cc2233",
	}
	statusItems := StatusItems{r: renderer{st: newStyles(Config{Palette: palette})}}

	tests := []struct {
		level string
		want  shared.Color
	}{
		{level: "", want: palette.Info},
		{level: "static", want: palette.Info},
		{level: "success", want: palette.Success},
		{level: "warning", want: palette.Warning},
		{level: "error", want: palette.Error},
	}
	for _, test := range tests {
		t.Run(test.level, func(t *testing.T) {
			got := statusItems.dot(test.level).GetForeground()
			want := lipgloss.Color(test.want.String())
			if !colorsEqual(got, want) {
				t.Fatalf("status %q color = %v, want %v", test.level, got, want)
			}
		})
	}
}
