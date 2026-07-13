package shared

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Palette is the JSON-bound color theme shared by menu-tui and motd.
// Nix generates identical palette JSON for both tools from the same
// theme/options. SelectionFg is only used by menu-tui; motd ignores it.
type Palette struct {
	Fg           Color `json:"fg"`
	Muted        Color `json:"muted"`
	Dim          Color `json:"dim"`
	Border       Color `json:"border"`
	AccentBorder Color `json:"accentBorder"`
	Accent       Color `json:"accent"`
	Accent2      Color `json:"accent2"`
	Error        Color `json:"error"`
	SelectionFg  Color `json:"selectionFg"`
	Bg           Color `json:"bg"`
	Surface      Color `json:"surface"`
	Secondary    Color `json:"secondary"`
}

// PaletteHelper provides color resolution and lipgloss style construction
// helpers shared by menu-tui and motd. Each tool builds its own styles struct
// from the palette; PaletteHelper centralizes the Color, Plain, and On
// closures used identically by both.
type PaletteHelper struct {
	Pal Palette
}

// NewPaletteHelper constructs a helper from a resolved palette.
func NewPaletteHelper(p Palette) PaletteHelper {
	return PaletteHelper{Pal: p}
}

// Color resolves a color string to a lipgloss color.
func (h PaletteHelper) Color(s string) color.Color {
	return lipgloss.Color(s)
}

// Plain returns a style with only the foreground set. This is the shared
// equivalent of the local `plain := func(fg string) lipgloss.Style { … }`
// closure both tools previously duplicated.
func (h PaletteHelper) Plain(fg string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(h.Color(fg))
}

// On returns a style with the given foreground placed on the given background.
// This is the shared equivalent of both tools' `on`/`onBlock`/`onBanner`
// closures.
func (h PaletteHelper) On(bg color.Color, fg string) lipgloss.Style {
	return h.Plain(fg).Background(bg)
}
