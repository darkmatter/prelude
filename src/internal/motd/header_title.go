package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// Title style variants for the header wordmark.
const (
	titleStylePlain     = "plain"
	titleStyleSpine     = "spine"
	titleStyleBracketed = "bracketed"
	titleStyleLabel     = "label"
	titleStyleInline    = "inline"   // project name centered in the accent gradient rule
	titleStyleInverted  = "inverted" // solid accent chip, selectionFg text
)

// HeaderTitle is a React-style, one-component-per-file component that renders
// the project wordmark on the card/page surface for non-inline styles.
type HeaderTitle struct{ r renderer }

// Render renders the project wordmark for the given style variant.
func (x HeaderTitle) Render(style string) string {
	name := x.r.cfg.Project
	dim, fg, accent := x.r.st.headerDim, x.r.st.headerFg, x.r.st.headerAccent
	switch strings.ToLower(style) {
	case titleStylePlain:
		return ui.Inline(accent).Bold(true).Render("  " + name + "  ")
	case titleStyleBracketed:
		return ui.Inline(dim).Render("  [ ") +
			ui.Inline(accent).Bold(true).Render(name) +
			ui.Inline(dim).Render(" ]  ")
	case titleStyleLabel:
		return ui.Inline(dim).Render("  devshell / ") +
			ui.Inline(fg).Bold(true).Render(name) +
			ui.Inline(dim).Render("  ")
	case titleStyleInverted:
		// Solid accent chip with dark/selection text — playground TitleInverted.
		chipFg := x.r.st.h.Color(string(x.r.st.pal.SelectionFg))
		if string(x.r.st.pal.SelectionFg) == "" {
			chipFg = x.r.st.h.Color(string(x.r.st.pal.Bg))
		}
		chipBg := x.r.st.h.Color(string(x.r.st.pal.Accent))
		return ui.Inline(lipgloss.NewStyle().Foreground(chipFg).Background(chipBg).Bold(true)).
			Render("  " + name + "  ")
	default: // spine
		return ui.Inline(accent).Render("  ▌ ") +
			ui.Inline(fg).Bold(true).Render(name) +
			ui.Inline(dim).Render("  ")
	}
}
