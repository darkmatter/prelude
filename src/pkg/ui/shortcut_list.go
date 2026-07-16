package ui

import "charm.land/lipgloss/v2"

// Shortcut is one command and optional alias in a ShortcutList.
type Shortcut struct {
	Command string
	Alias   string
}

// ShortcutList renders shortcut command chips as a horizontal list. Context
// supplies semantic defaults; each style optionally overrides one part.
type ShortcutList struct {
	Context       Context
	Items         []Shortcut
	Command       *lipgloss.Style
	Alias         *lipgloss.Style
	Separator     *lipgloss.Style
	SeparatorText string
}

func (x ShortcutList) commandStyle() lipgloss.Style {
	if x.Command != nil {
		return *x.Command
	}
	return x.Context.Muted()
}

func (x ShortcutList) aliasStyle() lipgloss.Style {
	if x.Alias != nil {
		return *x.Alias
	}
	return x.Context.Accent2().Bold(true)
}

func (x ShortcutList) keycap(alias string) string {
	bracket := x.Context.Dim()
	// return bracket.Render("[") + x.aliasStyle().Render(alias) + bracket.Render("]")
	return bracket.Render("[") + x.aliasStyle().Render(alias) + bracket.Render("]")
}

func (x ShortcutList) separatorStyle() lipgloss.Style {
	if x.Separator != nil {
		return *x.Separator
	}
	return x.Context.Dim()
}

// Render returns the styled shortcut chips, or an empty string when Items is empty.
func (x ShortcutList) Render() string {
	if len(x.Items) == 0 {
		return ""
	}

	separatorText := x.SeparatorText
	if separatorText == "" {
		separatorText = "   "
	}

	items := make([]string, 0, len(x.Items)*2-1)
	separator := Inline(x.separatorStyle()).Render(separatorText)
	for i, shortcut := range x.Items {
		if i > 0 {
			items = append(items, separator)
		}

		item := ""
		if shortcut.Alias != "" {
			item = x.keycap(shortcut.Alias)
		}
		if shortcut.Command != "" {
			if item != "" {
				item += x.Context.Fill().Render(" ")
			}
			item += Inline(x.commandStyle()).Render(shortcut.Command)
		}
		items = append(items, item)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, items...)
}
