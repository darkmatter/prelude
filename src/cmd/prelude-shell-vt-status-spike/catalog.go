package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
)

// menuCatalog is deliberately the small, JSON-facing subset of the Menu
// config. It stays local to this sibling prototype while reusable presentation
// components come from Prelude's public pkg/ui boundary.
type menuCatalog struct {
	Palette      menuPalette   `json:"palette"`
	Groups       []menuGroup   `json:"groups"`
	MOTDCommands []commandHint `json:"motdCommands"`

	commands []menuCommand
}

type menuPalette struct {
	Fg          string `json:"fg"`
	Muted       string `json:"muted"`
	Dim         string `json:"dim"`
	Accent      string `json:"accent"`
	Accent2     string `json:"accent2"`
	Success     string `json:"success"`
	Info        string `json:"info"`
	Warning     string `json:"warning"`
	Error       string `json:"error"`
	SelectionFg string `json:"selectionFg"`
	Bg          string `json:"bg"`
	Surface     string `json:"surface"`
	Secondary   string `json:"secondary"`
}

type commandHint struct {
	Name        string `json:"name"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

type menuGroup struct {
	Title string        `json:"title"`
	Tasks []menuCommand `json:"tasks"`
}

type menuCommand struct {
	Name        string    `json:"name"`
	Label       string    `json:"label"`
	Command     string    `json:"command"`
	Description string    `json:"description"`
	Usage       string    `json:"usage"`
	Details     string    `json:"details"`
	Examples    []string  `json:"examples"`
	Args        []menuArg `json:"args"`
}

type menuArg struct {
	Token       string   `json:"token"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Boolean     bool     `json:"boolean"`
	Options     []string `json:"options"`
}

type statusTheme struct {
	outer       color.Color
	bg          color.Color
	secondary   color.Color
	fg          color.Color
	muted       color.Color
	dim         color.Color
	accent      color.Color
	accent2     color.Color
	success     color.Color
	info        color.Color
	warning     color.Color
	error       color.Color
	selectionFg color.Color
}

func loadMenuCatalog(path string) (menuCatalog, error) {
	if path == "" {
		return menuCatalog{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return menuCatalog{}, fmt.Errorf("open PRELUDE_MENU_CONFIG: %w", err)
	}
	defer file.Close()

	var catalog menuCatalog
	if err := json.NewDecoder(file).Decode(&catalog); err != nil {
		return menuCatalog{}, fmt.Errorf("decode PRELUDE_MENU_CONFIG: %w", err)
	}
	for _, group := range catalog.Groups {
		for _, task := range group.Tasks {
			// Older configs do not have command yet. The fallback preserves a
			// useful status host while local Nix evaluation catches up.
			if task.Command == "" {
				if strings.Contains(task.Name, ":") {
					task.Command = "x " + task.Name
				} else {
					task.Command = task.Name
				}
			}
			catalog.commands = append(catalog.commands, task)
		}
	}
	if len(catalog.MOTDCommands) == 0 {
		// Backward-compatible fallback for an older generated config: show a
		// compact subset without reintroducing Starship's menu/docs shortcuts.
		for index, task := range catalog.commands {
			if index == 4 {
				break
			}
			catalog.MOTDCommands = append(catalog.MOTDCommands, commandHint{
				Name: task.Name, Command: task.Command, Description: task.Description,
			})
		}
	}
	return catalog, nil
}

func (catalog menuCatalog) commandAt(index int) (menuCommand, bool) {
	if index < 0 || index >= len(catalog.commands) {
		return menuCommand{}, false
	}
	return catalog.commands[index], true
}

func (catalog menuCatalog) statusTheme() statusTheme {
	defaults := statusTheme{
		outer:       color.RGBA{R: 12, G: 12, B: 19, A: 255},
		bg:          color.RGBA{R: 22, G: 22, B: 35, A: 255},
		secondary:   color.RGBA{R: 36, G: 36, B: 63, A: 255},
		fg:          color.RGBA{R: 228, G: 228, B: 231, A: 255},
		muted:       color.RGBA{R: 161, G: 161, B: 170, A: 255},
		dim:         color.RGBA{R: 100, G: 100, B: 110, A: 255},
		accent:      color.RGBA{R: 204, G: 153, B: 255, A: 255},
		accent2:     color.RGBA{R: 128, G: 180, B: 255, A: 255},
		success:     color.RGBA{R: 152, G: 195, B: 121, A: 255},
		info:        color.RGBA{R: 142, G: 202, B: 230, A: 255},
		warning:     color.RGBA{R: 229, G: 192, B: 123, A: 255},
		error:       color.RGBA{R: 238, G: 132, B: 142, A: 255},
		selectionFg: color.RGBA{R: 12, G: 12, B: 19, A: 255},
	}
	return statusTheme{
		outer:       parseHexColor(catalog.Palette.Bg, defaults.outer),
		bg:          parseHexColor(catalog.Palette.Surface, defaults.bg),
		secondary:   parseHexColor(catalog.Palette.Secondary, defaults.secondary),
		fg:          parseHexColor(catalog.Palette.Fg, defaults.fg),
		muted:       parseHexColor(catalog.Palette.Muted, defaults.muted),
		dim:         parseHexColor(catalog.Palette.Dim, defaults.dim),
		accent:      parseHexColor(catalog.Palette.Accent, defaults.accent),
		accent2:     parseHexColor(catalog.Palette.Accent2, defaults.accent2),
		success:     parseHexColor(catalog.Palette.Success, defaults.success),
		info:        parseHexColor(catalog.Palette.Info, defaults.info),
		warning:     parseHexColor(catalog.Palette.Warning, defaults.warning),
		error:       parseHexColor(catalog.Palette.Error, defaults.error),
		selectionFg: parseHexColor(catalog.Palette.SelectionFg, defaults.selectionFg),
	}
}

func parseHexColor(value string, fallback color.Color) color.Color {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 3 {
		value = strings.Repeat(string(value[0]), 2) + strings.Repeat(string(value[1]), 2) + strings.Repeat(string(value[2]), 2)
	}
	if len(value) != 6 {
		return fallback
	}
	number, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return fallback
	}
	return color.RGBA{R: uint8(number >> 16), G: uint8(number >> 8), B: uint8(number), A: 255}
}

// rgbaTo256 maps a color to the nearest 256-color cube index (16..231). This
// keeps the ble.sh palette portable without depending on truecolor support:
// the Go host resolves the Prelude hex palette to indices once, and the sourced
// ble.sh integration only ever sees integers.
func rgbaTo256(c color.Color) int {
	r, g, b, _ := c.RGBA()
	const scale = 6
	levels := [scale]int{0, 95, 135, 175, 215, 255}
	cr := nearestCubeLevel(int(r>>8), levels)
	cg := nearestCubeLevel(int(g>>8), levels)
	cb := nearestCubeLevel(int(b>>8), levels)
	return 16 + 36*cr + 6*cg + cb
}

func nearestCubeLevel(component int, levels [6]int) int {
	best, bestDistance := 0, abs(component-levels[0])
	for i := 1; i < len(levels); i++ {
		distance := abs(component - levels[i])
		if distance < bestDistance {
			best, bestDistance = i, distance
		}
	}
	return best
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// emitShellCatalog writes a shell-sourceable fragment carrying the catalog and
// the resolved palette as 256-color indices. ble.sh-side logic lives in
// prelude-ble.sh; this file is pure data so it can be regenerated from
// PRELUDE_MENU_CONFIG without touching the integration script.
func (catalog menuCatalog) emitShellCatalog() []byte {
	theme := catalog.statusTheme()
	palette := []struct {
		name  string
		value color.Color
	}{
		{"fg", theme.fg}, {"muted", theme.muted}, {"dim", theme.dim},
		{"accent", theme.accent}, {"accent2", theme.accent2}, {"success", theme.success},
		{"info", theme.info}, {"warning", theme.warning}, {"error", theme.error},
		{"selectionfg", theme.selectionFg}, {"bg", theme.bg}, {"surface", theme.bg},
		{"secondary", theme.secondary},
	}
	var b strings.Builder
	b.WriteString("# prelude-catalog.sh — generated by prelude-shell-vt-status-spike\n")
	for _, role := range palette {
		fmt.Fprintf(&b, "prelude_palette_%s=%d\n", role.name, rgbaTo256(role.value))
	}
	b.WriteString("\nprelude_catalog_names=(")
	for _, command := range catalog.commands {
		b.WriteString(" " + shellSingle(command.Name))
	}
	b.WriteString(" )\nprelude_catalog_commands=(")
	for _, command := range catalog.commands {
		b.WriteString(" " + shellSingle(command.Command))
	}
	b.WriteString(" )\nprelude_catalog_descriptions=(")
	for _, command := range catalog.commands {
		description := command.Description
		if description == "" {
			description = command.Command
		}
		b.WriteString(" " + shellSingle(description))
	}
	b.WriteString(" )\nprelude_catalog_argopts=(")
	for _, command := range catalog.commands {
		b.WriteString(" " + shellSingle(strings.Join(commandArgOptions(command), " ")))
	}
	b.WriteString(" )\nprelude_direct_commands=(")
	for _, command := range catalog.commands {
		if command.Command != "" && !strings.ContainsAny(command.Command, " \t") {
			b.WriteString(" " + shellSingle(command.Command))
		}
	}
	b.WriteString(" )\n")
	return []byte(b.String())
}

func commandArgOptions(command menuCommand) []string {
	var candidates []string
	for _, argument := range command.Args {
		if strings.HasPrefix(argument.Token, "-") {
			candidates = append(candidates, argument.Token)
		}
		candidates = append(candidates, argument.Options...)
	}
	return uniqueStrings(candidates)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// shellSingle single-quotes a value for shell contexts. The replacement is the
// standard '\” idiom: close the single quote, insert an escaped single quote,
// reopen the single quote.
func shellSingle(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
