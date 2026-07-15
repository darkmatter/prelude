package main

import (
	"fmt"
	"image/color"

	"charm.land/lipgloss/v2"
)

func needsRelativeBackgrounds(cfg Config) bool {
	return cfg.BackgroundRelative != 0 || cfg.BackgroundBlendSet ||
		cfg.WindowBackgroundRelative != 0 || cfg.WindowBackgroundBlendSet ||
		cfg.Description.BackgroundRelative != 0 ||
		cfg.Header.BackgroundRelative != 0
}

func needsTerminalBackground(cfg Config) bool {
	// Any window background needs the terminal color to fade the margins.
	if cfg.WindowBackground != "" {
		return true
	}
	// Description-relative can use an explicit card color; only card/window/
	// header relative values (or description with no card color) need a query.
	if cfg.BackgroundRelative != 0 || cfg.BackgroundBlendSet ||
		cfg.WindowBackgroundRelative != 0 || cfg.WindowBackgroundBlendSet ||
		cfg.Header.BackgroundRelative != 0 {
		return true
	}
	// A transparent card with codeblocks shades the codeblock surface from
	// the terminal color, so the query is still required.
	if cfg.Background == "" && len(cfg.Recipes) > 0 {
		return true
	}
	return cfg.Description.BackgroundRelative != 0 && cfg.Background == ""
}

// resolveRelativeBackgrounds converts runtime-relative background values into
// concrete colors before styles are built. Card/window/header values use the
// detected terminal background; nested description values use the resolved card.
func resolveRelativeBackgrounds(cfg Config, terminalBackground color.Color) Config {
	terminalBase := terminalBackground
	if terminalBase == nil {
		terminalBase = lipgloss.Color(string(cfg.Palette.Bg))
	}
	cfg.TerminalBackground = colorHex(terminalBase)

	if cfg.BackgroundRelative != 0 {
		cfg.Background = relativeColor(terminalBase, cfg.BackgroundRelative)
	}
	if cfg.BackgroundBlendSet {
		cfg.Background = blendColor(terminalBase, cfg.Palette.Bg.String(), cfg.BackgroundBlend)
	}
	if cfg.WindowBackgroundRelative != 0 {
		cfg.WindowBackground = relativeColor(terminalBase, cfg.WindowBackgroundRelative)
	}
	if cfg.WindowBackgroundBlendSet {
		cfg.WindowBackground = blendColor(terminalBase, cfg.Palette.Bg.String(), cfg.WindowBackgroundBlend)
	}
	if cfg.Header.BackgroundRelative != 0 {
		cfg.Header.Background = relativeColor(terminalBase, cfg.Header.BackgroundRelative)
		cfg.Header.BackgroundRaised = false
	}
	if cfg.Description.BackgroundRelative != 0 {
		cardBase := color.Color(terminalBase)
		if cfg.Background != "" {
			cardBase = lipgloss.Color(cfg.Background)
		}
		cfg.Description.Background = relativeColor(cardBase, cfg.Description.BackgroundRelative)
	}
	return cfg
}

func blendColor(terminalBase color.Color, themeBg string, amount float64) string {
	steps := lipgloss.Blend1D(101, terminalBase, lipgloss.Color(themeBg))
	return colorHex(steps[int(amount*100)])
}

func relativeColor(base color.Color, amount float64) string {
	adjusted := base
	if amount < 0 {
		adjusted = lipgloss.Darken(base, -amount)
	} else if amount > 0 {
		adjusted = lipgloss.Lighten(base, amount)
	}
	return colorHex(adjusted)
}

func colorHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
