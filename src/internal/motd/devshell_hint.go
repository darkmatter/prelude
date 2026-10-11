package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// DevshellHintView uses the card geometry without extending its background.
type DevshellHintView struct{ r renderer }

func (v DevshellHintView) Render() string {
	context := ui.NewContext(v.r.model.Config.Palette, nil, true)
	muted := ui.Inline(context.Muted())
	text := muted.Render("You're using the cached devshell, which doesn't run the shellHook in your interactive shell. Run ") +
		ui.Inline(context.Accent()).Render("nix develop") +
		muted.Render(" for the full devshell.")

	var rows []string
	for _, line := range ui.WrapText(text, v.r.contentWidth) {
		rows = append(rows, ui.PlaceContentLine(line, v.r.cardWidth, v.r.contentWidth,
			v.r.model.Padding.Left, lipgloss.Center, context.Fill()))
	}
	return strings.Join(rows, "\n")
}
