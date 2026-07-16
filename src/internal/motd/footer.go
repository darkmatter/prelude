package motd

import (
	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// FooterView owns the MOTD's closing row. A generated title uses it for
// resolved status badges; otherwise it exposes configured shortcut hints.
type FooterView struct{ r renderer }

// Render places status badges in generated-title layouts, optionally split
// left/right with the async hint. Other layouts right-align shortcut hints.
func (x FooterView) Render() string {
	if x.r.cfg.Title != "" && x.r.cfg.Header.StatusHintLayout == "inline" {
		if row := (StatusItems{r: x.r}).InlineHint(x.r.cfg.Header.Status, x.r.contentWidth, false); row != "" {
			return ui.PlaceContentLine(
				row,
				x.r.cardWidth,
				x.r.contentWidth,
				x.r.cfg.Padding.Left,
				lipgloss.Left,
				x.r.st.blockFill,
			)
		}
	}

	content, align := x.shortcuts(), lipgloss.Right
	if x.r.cfg.Title != "" {
		content, align = x.statusItems(), lipgloss.Center
	}
	if content == "" {
		return ""
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
		return line
	}
	hint := (StatusItems{r: x.r}).Hint(false)
	if hint == "" {
		return line
	}
	return line + "\n\n" + ui.PlaceContentLine(
		hint,
		x.r.cardWidth,
		x.r.contentWidth,
		x.r.cfg.Padding.Left,
		lipgloss.Center,
		x.r.st.blockFill,
	)
}

func (x FooterView) statusItems() string {
	status := (StatusItems{r: x.r}).Render(x.r.cfg.Header.Status, false)
	if lipgloss.Width(status) <= x.r.contentWidth {
		return status
	}
	return (StatusItems{r: x.r}).Render(x.r.cfg.Header.Status, true)
}

func (x FooterView) shortcuts() string {
	return (Shortcuts{r: x.r}).Render()
}
