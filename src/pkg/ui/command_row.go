package ui

import "charm.land/lipgloss/v2"

// CommandRow renders a command with dotted leaders and a right-aligned
// description. Context supplies semantic defaults; individual styles may be
// overridden by callers for deliberate local exceptions.
type CommandRow struct {
	Context          Context
	Command          string
	Description      string
	Width            int
	Prompt           *lipgloss.Style
	CommandStyle     *lipgloss.Style
	DescriptionStyle *lipgloss.Style
	Leader           *lipgloss.Style
	Fill             *lipgloss.Style
}

func (x CommandRow) promptStyle() lipgloss.Style {
	if x.Prompt != nil {
		return *x.Prompt
	}
	return x.Context.Accent()
}

func (x CommandRow) commandStyle() lipgloss.Style {
	if x.CommandStyle != nil {
		return *x.CommandStyle
	}
	return x.Context.Foreground().Bold(true)
}

func (x CommandRow) descriptionStyle() lipgloss.Style {
	if x.DescriptionStyle != nil {
		return *x.DescriptionStyle
	}
	return x.Context.Muted()
}

func (x CommandRow) leaderStyle() lipgloss.Style {
	if x.Leader != nil {
		return *x.Leader
	}
	return x.Context.Dim()
}

func (x CommandRow) fillStyle() lipgloss.Style {
	if x.Fill != nil {
		return *x.Fill
	}
	return x.Context.Fill()
}

// Render returns the styled command row, including one-cell spacer regions on
// either side of the dotted leaders and fill to Width.
func (x CommandRow) Render() string {
	left := Inline(x.promptStyle()).Render("$ ") +
		Inline(x.commandStyle()).Render(x.Command)
	right := Inline(x.descriptionStyle()).Render(x.Description)

	leaderWhitespace := lipgloss.WithWhitespaceStyle(x.leaderStyle())
	space := lipgloss.PlaceHorizontal(1, lipgloss.Left, "", leaderWhitespace)
	dots := lipgloss.PlaceHorizontal(
		max(x.Width-lipgloss.Width(left)-lipgloss.Width(right)-2, 1),
		lipgloss.Left,
		"",
		lipgloss.WithWhitespaceChars("·"),
		leaderWhitespace,
	)
	line := lipgloss.JoinHorizontal(lipgloss.Top, left, space, dots, space, right)
	return x.fillStyle().Width(x.Width).Render(line)
}
