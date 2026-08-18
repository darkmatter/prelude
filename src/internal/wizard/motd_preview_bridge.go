package wizard

import (
	"strings"

	"prelude/internal/motd"
	"prelude/pkg/shared"
)

// motdPreviewConfig builds a motd.Config from the wizard's current state so
// the wizard preview is produced by the real MOTDView renderer (the same code
// path that paints the banner on shell entry), not a hand-rolled approximation.
func (m wizardModel) motdPreviewConfig() motd.Config {
	margin := motdMarginPresets[boundedIndex(m.motdMarginIndex, len(motdMarginPresets))]
	padding := motdPaddingPresets[boundedIndex(m.motdPaddingIndex, len(motdPaddingPresets))]
	width := motdWidthPresets[boundedIndex(m.motdWidthIndex, len(motdWidthPresets))]
	theme := m.selectedTheme()

	background := m.previewBackgroundHex()

	statuses := m.previewHeaderStatuses()
	commands := m.previewCommands()

	return motd.Config{
		Project:      m.sampleProject(),
		Title:        strings.TrimSpace(m.motdWordmark),
		TitleAlign:   motdTitleAlignments[boundedIndex(m.motdTitleAlignIndex, len(motdTitleAlignments))],
		ColorProfile: "auto",
		Palette:      wizardPalette(theme.Palette),
		Background:   background,
		Border:       m.motdBorder,
		// The preview lives inside the wizard panel, so never emit the
		// clear-screen control sequences. The meta line still reports the
		// configured clear-screen choice.
		ClearScreen: false,
		Margin: motd.Spacing{
			Left:      margin.X,
			Right:     margin.X,
			Top:       margin.Y,
			Bottom:    margin.Y,
			MinHeight: margin.MinHeight,
		},
		Align:         motdAlignments[boundedIndex(m.motdAlignIndex, len(motdAlignments))],
		VerticalAlign: motdVerticalAlignments[boundedIndex(m.motdVerticalAlignIndex, len(motdVerticalAlignments))],
		Padding: motd.Spacing{
			Left:      padding.X,
			Right:     padding.X,
			Top:       padding.Y,
			Bottom:    padding.Y,
			MinHeight: padding.MinHeight,
		},
		Header: motd.Header{
			Tagline:       strings.TrimSpace(m.motdContent.Tagline),
			TaglineLayout: "stack",
			Status:        statuses,
		},
		Description: motd.StyledText{
			Text: strings.TrimSpace(m.motdContent.Description),
		},
		Commands:       commands,
		GettingStarted: motd.GettingStarted{Heading: "Getting Started", CommandsLabel: "commands"},
		Width:          width.WidthInt,
		MaxWidth:       width.MaxWidthInt,
	}
}

// previewBackgroundHex maps the wizard's background-mode choice to the hex
// color the motd.Config expects: empty means transparent (let the terminal show
// through), otherwise the theme token color for theme/surface/accent modes.
func (m wizardModel) previewBackgroundHex() string {
	option := motdBackgroundOptions[boundedIndex(m.motdBackgroundIndex, len(motdBackgroundOptions))]
	if option.Token == "" {
		return ""
	}
	// Pull the hex string directly from the theme palette so motd.Config gets
	// a literal color value rather than a Nix-shaped expression.
	if hex, ok := m.selectedTheme().Palette[option.Token]; ok && hex != "" {
		return hex
	}
	return ""
}

// previewHeaderStatuses builds the motd.HeaderStatus list from the wizard's
// toggles. Checks are async and render "pending" until the cache resolves.
func (m wizardModel) previewHeaderStatuses() []motd.HeaderStatus {
	var statuses []motd.HeaderStatus
	if m.motdContent.NixFlakeCheck {
		statuses = append(statuses, motd.HeaderStatus{
			Label: "flake",
			Check: "nix flake check",
			Async: true,
			Ok:    "ok",
			Fail:  "fail",
		})
	}
	if m.motdContent.DevServerStatus {
		statuses = append(statuses, motd.HeaderStatus{
			Label: "dev server",
			Check: motdDevServerHealthCommand(m.motdContent.DevServerHealthURL),
			Async: true,
			Ok:    "ok",
			Fail:  "fail",
		})
	}
	return statuses
}

// previewCommands converts the wizard's command list to motd.Command entries
// for the Getting Started region.
func (m wizardModel) previewCommands() []motd.Command {
	if len(m.commands) == 0 {
		return nil
	}
	count := min(len(m.commands), 3)
	commands := make([]motd.Command, count)
	for i := 0; i < count; i++ {
		c := m.commands[i]
		exec := c.Exec
		if exec == "" {
			exec = c.Name
		}
		commands[i] = motd.Command{Command: exec, Description: c.Description}
	}
	return commands
}

// wizardPalette converts the wizard's loose map[string]string theme palette
// into the strongly-typed shared.Palette the motd Config expects.
func wizardPalette(tokens map[string]string) shared.Palette {
	return shared.Palette{
		Fg:           shared.Color(tokens["fg"]),
		Muted:        shared.Color(tokens["muted"]),
		Dim:          shared.Color(tokens["dim"]),
		Border:       shared.Color(tokens["border"]),
		AccentBorder: shared.Color(tokens["accentBorder"]),
		Accent:       shared.Color(tokens["accent"]),
		Accent2:      shared.Color(tokens["accent2"]),
		Success:      shared.Color(tokens["success"]),
		Warning:      shared.Color(tokens["warning"]),
		Info:         shared.Color(tokens["info"]),
		Error:        shared.Color(tokens["error"]),
		SelectionFg:  shared.Color(tokens["selectionFg"]),
		Bg:           shared.Color(tokens["bg"]),
		Surface:      shared.Color(tokens["surface"]),
		Secondary:    shared.Color(tokens["secondary"]),
	}
}
