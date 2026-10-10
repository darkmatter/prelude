package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type chromePalette struct {
	Fg      color.Color
	Bg      color.Color
	Success color.Color
	Muted   color.Color
	Accent  color.Color
	Accent2 color.Color
	Warning color.Color
	Error   color.Color
}

// Nix owns the palette. Missing colors stay nil so chrome inherits the terminal
// defaults instead of introducing a second set of theme defaults in the host.
func loadChromePalette(env []string) (chromePalette, error) {
	var palette chromePalette
	path := environmentValue(env, "PRELUDE_MENU_CONFIG")
	if path == "" {
		return palette, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return palette, fmt.Errorf("PRELUDE_MENU_CONFIG %q: read menu config: %w", path, err)
	}
	var config struct {
		Palette map[string]json.RawMessage `json:"palette"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return palette, fmt.Errorf("PRELUDE_MENU_CONFIG %q: decode menu config: %w", path, err)
	}
	if config.Palette == nil {
		return palette, fmt.Errorf("PRELUDE_MENU_CONFIG %q: expected a menu config with a .palette object", path)
	}
	for _, token := range []struct {
		name  string
		color *color.Color
	}{
		{"fg", &palette.Fg},
		{"bg", &palette.Bg},
		{"success", &palette.Success},
		{"muted", &palette.Muted},
		{"accent", &palette.Accent},
		{"accent2", &palette.Accent2},
		{"warning", &palette.Warning},
		{"error", &palette.Error},
	} {
		value, present := config.Palette[token.name]
		if !present {
			continue
		}
		parsed, err := parseChromeColor(value)
		if err != nil {
			return chromePalette{}, fmt.Errorf("PRELUDE_MENU_CONFIG %q: .palette.%s: %w", path, token.name, err)
		}
		*token.color = parsed
	}
	return palette, nil
}

func environmentValue(env []string, name string) string {
	for i := len(env) - 1; i >= 0; i-- {
		key, value, _ := strings.Cut(env[i], "=")
		if key == name {
			return value
		}
	}
	return ""
}

func parseChromeColor(value json.RawMessage) (color.Color, error) {
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		var number json.Number
		if err := json.Unmarshal(value, &number); err != nil {
			return nil, fmt.Errorf("expected a hex string or ANSI256 index (0..255), got %s", value)
		}
		text = number.String()
	}
	// Like shared.Color, null and empty strings represent an unspecified color.
	if text == "" {
		return nil, nil
	}
	if strings.HasPrefix(text, "#") {
		hex := ansi.HexColor(text)
		if hex.Hex() == "" {
			return nil, fmt.Errorf("invalid hex color %q: expected #RGB or #RRGGBB", text)
		}
		return hex, nil
	}
	index, err := strconv.Atoi(text)
	if err != nil || index < 0 || index > 255 {
		return nil, fmt.Errorf("invalid color %q: expected #RGB, #RRGGBB, or an ANSI256 index (0..255)", text)
	}
	if index < 16 {
		return ansi.BasicColor(index), nil
	}
	return ansi.IndexedColor(index), nil
}
