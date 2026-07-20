package motd

import (
	"image/color"
	"time"

	"charm.land/lipgloss/v2"
)

// RenderInput is the single pure input to Render: Config plus Cache.
// TerminalWidth/Height override cache/defaults when positive (tests / pure inject).
type RenderInput struct {
	Config         Config
	Cache          Cache
	TerminalWidth  int
	TerminalHeight int
}

// Render produces the MOTD banner purely from RenderInput.
// Missing/stale cache yields sparse UI (P1); never fails for live data absence.
// Post-banner diagnostics are not emitted (D2).
func Render(in RenderInput) string {
	cfg := applyCache(in.Config, in.Cache, time.Now())
	width, height := resolveTerminalSize(in)
	return render(cfg, width, height)
}

func resolveTerminalSize(in RenderInput) (width, height int) {
	if in.TerminalWidth > 0 {
		width = in.TerminalWidth
	}
	if in.TerminalHeight > 0 {
		height = in.TerminalHeight
	}
	if width == 0 || height == 0 {
		if e, ok := in.Cache.entry(keyTerminalSize); ok {
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
	return width, height
}

// applyCache returns a paint-ready Config copy with live fields filled from Cache.
func applyCache(cfg Config, cache Cache, now time.Time) Config {
	// Backgrounds from cached terminal color.
	if needsRelativeBackgrounds(cfg) || needsTerminalBackground(cfg) {
		var terminalBackground color.Color
		if e, ok := cache.entry(keyTerminalBG); ok && e.Color != "" {
			terminalBackground = lipgloss.Color(e.Color)
		}
		cfg = resolveRelativeBackgrounds(cfg, terminalBackground)
	}

	var oldestAsync time.Time
	asyncFound := false
	hasAsync := false

	for i := range cfg.Header.Status {
		item := &cfg.Header.Status[i]
		if item.Check == "" {
			if item.Status != "" || item.Label != "" {
				item.Level = "static"
			}
			continue
		}
		key := statusKey(item.Check)
		if item.Async {
			hasAsync = true
			e, ok := cache.entry(key)
			if !ok || (e.Status == "" && e.Level == "") {
				item.Status = "pending"
				item.Level = "static"
				continue
			}
			item.Status, item.Level = e.Status, e.Level
			if !e.CheckedAt.IsZero() {
				if !asyncFound || e.CheckedAt.Before(oldestAsync) {
					oldestAsync = e.CheckedAt
					asyncFound = true
				}
			}
			continue
		}
		// Sync: use cache when present; otherwise leave empty (preflight should have filled).
		if e, ok := cache.entry(key); ok {
			item.Status, item.Level = e.Status, e.Level
		}
	}

	if hasAsync {
		cfg.StatusHint = "[r] to reload"
		if asyncFound {
			age := naturalAge(now.Sub(oldestAsync))
			if age == "just now" {
				cfg.StatusAge = "just now"
			} else {
				cfg.StatusAge = age + " ago"
			}
		} else {
			cfg.StatusAge = ""
		}
	}

	for i := range cfg.Env {
		item := &cfg.Env[i]
		if item.Probe == "" {
			continue
		}
		if e, ok := cache.entry(envKey(item.Probe)); ok {
			item.Value = e.Value
		} else {
			// Cold cache: omit chip by clearing value (Env.Render skips empty probe results).
			item.Value = ""
		}
	}

	return cfg
}
