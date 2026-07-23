package motd

import (
	"fmt"
	"image/color"
	"time"

	"charm.land/lipgloss/v2"
)

// PaintModel is the resolved, pure input to Render. It holds the original
// Config plus all live-derived values: cache-resolved statuses and env,
// terminal-derived backgrounds, and terminal geometry. Nothing in PaintModel
// is mutated after Resolve returns.
type PaintModel struct {
	Config Config

	// TerminalBackground is the detected terminal background color, used as the
	// base for window-background fades and relative blends.
	TerminalBackground string

	// Background is the resolved card background (explicit, relative, or blended).
	Background string
	// WindowBackground is the resolved window/screen background.
	WindowBackground string
	// HeaderBackground is the resolved header bar background.
	HeaderBackground string
	// DescriptionBackground is the resolved description section background.
	DescriptionBackground string

	// Status is the cache-resolved header status list.
	Status []ResolvedStatus
	// StatusHint is the async refresh hint (e.g. "[r] to reload").
	StatusHint string
	// StatusAge is the human-readable age of the oldest cached async status.
	StatusAge string

	// Env is the cache-resolved env list.
	Env []ResolvedEnv

	TerminalWidth    int
	TerminalHeight   int
	Margin           Spacing
	Padding          Spacing
	CardWidth        int
	ContentWidth     int
	HorizontalOffset int
}

// ResolvedStatus is one cache-resolved header status badge.
type ResolvedStatus struct {
	Label  string
	Status string
	Level  string
}

// ResolvedEnv is one cache-resolved env chip.
type ResolvedEnv struct {
	Label string
	Value string
}

// Resolve builds a PaintModel from Config and Cache. It is pure except for the
// optional terminal size/background overrides passed by the caller.
func Resolve(cfg Config, cache Cache, terminalWidth, terminalHeight int, now time.Time) PaintModel {
	width, height := resolveTerminalSize(cache, terminalWidth, terminalHeight)
	terminalBG := resolveTerminalBackground(cache)

	background, windowBackground, headerBackground, descriptionBackground :=
		resolveBackgrounds(cfg, terminalBG)

	margin := cfg.Margin.collapseVertical(height)
	padding := cfg.Padding.collapseVertical(height)

	status, statusHint, statusAge := resolveStatus(cfg.Header.Status, cache, now)
	env := resolveEnv(cfg.Env, cache)

	cardWidth := resolveCardWidth(cfg.Width, cfg.MaxWidth, width)
	contentWidth := max(cardWidth-max(padding.Left, 0)-max(padding.Right, 0), 1)

	model := PaintModel{
		Config:                cfg,
		TerminalBackground:    terminalBG,
		Background:            background,
		WindowBackground:      windowBackground,
		HeaderBackground:      headerBackground,
		DescriptionBackground: descriptionBackground,
		Status:                status,
		StatusHint:            statusHint,
		StatusAge:             statusAge,
		Env:                   env,
		TerminalWidth:         width,
		TerminalHeight:        height,
		Margin:                margin,
		Padding:               padding,
		CardWidth:             cardWidth,
		ContentWidth:          contentWidth,
	}
	model.HorizontalOffset = model.resolveHorizontalOffset()
	return model
}

func resolveTerminalSize(cache Cache, terminalWidth, terminalHeight int) (width, height int) {
	width = terminalWidth
	height = terminalHeight
	if width == 0 || height == 0 {
		if e, ok := cache.entry(keyTerminalSize); ok {
			if width == 0 && e.Width > 0 {
				width = e.Width
			}
			if height == 0 && e.Height > 0 {
				height = e.Height
			}
		}
	}
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return width, max(height, 1)
}

func resolveTerminalBackground(cache Cache) string {
	if e, ok := cache.entry(keyTerminalBG); ok && e.Color != "" {
		return e.Color
	}
	return ""
}

func resolveBackgrounds(cfg Config, terminalBackground string) (background, windowBackground, headerBackground, descriptionBackground string) {
	terminalBase := lipgloss.Color(terminalBackground)
	if terminalBase == nil {
		terminalBase = lipgloss.Color(string(cfg.Palette.Bg))
	}

	background = cfg.Background
	if cfg.BackgroundRelative != 0 {
		background = relativeColor(terminalBase, cfg.BackgroundRelative)
	}
	if cfg.BackgroundBlendSet {
		background = blendColor(terminalBase, cfg.Palette.Bg.String(), cfg.BackgroundBlend)
	}

	windowBackground = cfg.WindowBackground
	if cfg.WindowBackgroundRelative != 0 {
		windowBackground = relativeColor(terminalBase, cfg.WindowBackgroundRelative)
	}
	if cfg.WindowBackgroundBlendSet {
		windowBackground = blendColor(terminalBase, cfg.Palette.Bg.String(), cfg.WindowBackgroundBlend)
	}

	headerBackground = cfg.Header.Background
	if cfg.Header.BackgroundRelative != 0 {
		headerBackground = relativeColor(terminalBase, cfg.Header.BackgroundRelative)
	}

	descriptionBackground = cfg.Description.Background
	if cfg.Description.BackgroundRelative != 0 {
		cardBase := terminalBase
		if background != "" {
			cardBase = lipgloss.Color(background)
		}
		descriptionBackground = relativeColor(cardBase, cfg.Description.BackgroundRelative)
	}

	return
}

func resolveStatus(items []HeaderStatus, cache Cache, now time.Time) (status []ResolvedStatus, hint, age string) {
	status = make([]ResolvedStatus, 0, len(items))

	var oldestAsync time.Time
	asyncFound := false
	hasAsync := false

	for _, item := range items {
		if item.Check == "" {
			if item.Status != "" || item.Label != "" {
				status = append(status, ResolvedStatus{Label: item.Label, Status: item.Status, Level: "static"})
			}
			continue
		}

		key := statusKey(item.Check)
		if item.Async {
			hasAsync = true
			e, ok := cache.entry(key)
			if !ok || (e.Status == "" && e.Level == "") {
				status = append(status, ResolvedStatus{Label: item.Label, Status: "pending", Level: "static"})
				continue
			}
			status = append(status, ResolvedStatus{Label: item.Label, Status: e.Status, Level: e.Level})
			if !e.CheckedAt.IsZero() {
				if !asyncFound || e.CheckedAt.Before(oldestAsync) {
					oldestAsync = e.CheckedAt
					asyncFound = true
				}
			}
			continue
		}

		resolved := ResolvedStatus{Label: item.Label, Level: "static"}
		if e, ok := cache.entry(key); ok {
			resolved.Status = e.Status
			resolved.Level = e.Level
		}
		status = append(status, resolved)
	}

	if hasAsync {
		hint = "[r] to reload"
		if asyncFound {
			ageText := naturalAge(now.Sub(oldestAsync))
			if ageText == "just now" {
				age = "just now"
			} else {
				age = ageText + " ago"
			}
		}
	}

	return status, hint, age
}

func resolveEnv(items []EnvItem, cache Cache) []ResolvedEnv {
	resolved := make([]ResolvedEnv, 0, len(items))
	for _, item := range items {
		value := item.Value
		if item.Probe != "" {
			if e, ok := cache.entry(envKey(item.Probe)); ok {
				value = e.Value
			} else {
				value = ""
			}
		}
		resolved = append(resolved, ResolvedEnv{Label: item.Label, Value: value})
	}
	return resolved
}

func resolveCardWidth(width, maxWidth, terminalWidth int) int {
	cardWidth := width
	if cardWidth == 0 {
		cardWidth = terminalWidth
	}
	if maxWidth > 0 && cardWidth > maxWidth {
		cardWidth = maxWidth
	}
	return max(cardWidth, minimumCardWidth)
}

func (m PaintModel) resolveHorizontalOffset() int {
	switch m.Config.Align {
	case "right":
		return max(m.TerminalWidth-m.CardWidth-m.Margin.Right, 0)
	case "center":
		return max((m.TerminalWidth-m.CardWidth)/2+m.Margin.Left-m.Margin.Right, 0)
	default:
		return max(m.Margin.Left, 0)
	}
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
