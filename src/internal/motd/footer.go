package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// FooterView owns the MOTD's configured links. Status badges belong to the
// header for every title layout, so links remain the final centered rows.
type FooterView struct{ r renderer }

func (x FooterView) Render() string {
	return x.linksRow()
}

// linksRow renders configured links as one centered content line per wrapped
// label line, joined directly (no blank line between consecutive links).
// Returns "" when there are no links so Render can skip appending it.
func (x FooterView) linksRow() string {
	linkLines := sections{r: x.r}.links()
	if len(linkLines) == 0 {
		return ""
	}
	var lines []string
	for _, rendered := range linkLines {
		lines = append(lines, ui.PlaceContentLine(
			rendered,
			x.r.cardWidth,
			x.r.contentWidth,
			x.r.model.Padding.Left,
			lipgloss.Center,
			x.r.st.blockFill,
		))
	}
	return strings.Join(lines, "\n")
}
