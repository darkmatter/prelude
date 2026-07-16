// Package ui provides shared lipgloss rendering utilities used by the
// interactive menu, the MOTD renderer, and the manual viewer.
package ui

import "charm.land/lipgloss/v2"

// Inline strips layout from a style so only paint applies when joining segments.
func Inline(st lipgloss.Style) lipgloss.Style {
	return st.Inline(true)
}
