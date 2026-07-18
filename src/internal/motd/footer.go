package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// FooterView owns the MOTD's closing rows for resolved status badges, the async
// hint, and configured links. Links always render as the final centered row so
// the URL sits at the very bottom of the card regardless of which status
// variant is active.
type FooterView struct{ r renderer }

// Render places status badges in generated-title layouts, optionally split
// left/right with the async hint, followed by a centered links row.
func (x FooterView) Render() string {
	rows := x.statusRows()
	if linkRow := x.linksRow(); linkRow != "" {
		rows = append(rows, linkRow)
	}
	return strings.Join(rows, "\n\n")
}

// statusRows emits the status badges and optional async hint. It mirrors the
// previous single-string Render output without the links row, returning a
// slice so links can be appended unconditionally by Render.
func (x FooterView) statusRows() []string {
	if x.r.cfg.Title != "" && x.r.cfg.Header.StatusHintLayout == "inline" {
		if row := (StatusItems{r: x.r}).InlineHint(x.r.cfg.Header.Status, x.r.contentWidth, false); row != "" {
			return []string{ui.PlaceContentLine(
				row,
				x.r.cardWidth,
				x.r.contentWidth,
				x.r.cfg.Padding.Left,
				lipgloss.Left,
				x.r.st.blockFill,
			)}
		}
	}

	content, align := "", lipgloss.Center
	if x.r.cfg.Title != "" {
		content = x.statusItems()
	}
	if content == "" {
		return nil
	}

	line := ui.PlaceContentLine(
		content,
		x.r.cardWidth,
		x.r.contentWidth,
		x.r.cfg.Padding.Left,
		align,
		x.r.st.blockFill,
	)
	if x.r.cfg.Title == "" {
		return []string{line}
	}
	hint := (StatusItems{r: x.r}).Hint(false)
	if hint == "" {
		return []string{line}
	}
	return []string{
		line,
		ui.PlaceContentLine(
			hint,
			x.r.cardWidth,
			x.r.contentWidth,
			x.r.cfg.Padding.Left,
			lipgloss.Center,
			x.r.st.blockFill,
		),
	}
}

// linksRow renders configured links as one centered content line per wrapped
// label line, joined directly (no blank line between consecutive links).
// Returns "" when there are no links so Render can skip appending it.
func (x FooterView) linksRow() string {
	if len(x.r.cfg.Links) == 0 {
		return ""
	}
	var lines []string
	for _, link := range x.r.cfg.Links {
		for _, labelLine := range ui.WrapText(link.Label, x.r.contentWidth) {
			rendered := (ui.Link{
				Context: x.r.blockUI,
				Label:   labelLine,
				URL:     link.URL,
			}).Render()
			if rendered == "" {
				continue
			}
			lines = append(lines, ui.PlaceContentLine(
				rendered,
				x.r.cardWidth,
				x.r.contentWidth,
				x.r.cfg.Padding.Left,
				lipgloss.Center,
				x.r.st.blockFill,
			))
		}
	}
	return strings.Join(lines, "\n")
}

func (x FooterView) statusItems() string {
	status := (StatusItems{r: x.r}).Render(x.r.cfg.Header.Status, false)
	if lipgloss.Width(status) <= x.r.contentWidth {
		return status
	}
	return (StatusItems{r: x.r}).Render(x.r.cfg.Header.Status, true)
}
