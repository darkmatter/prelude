package motd

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestStatusItemsInlineHintAlignsStatusAndHint(t *testing.T) {
	r := renderer{cfg: Config{StatusHint: "17 m ago • [r] to reload"}}
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
