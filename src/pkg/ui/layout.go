package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// PlaceRight places right content flush against the right edge of width,
// with left content at the left edge and filler bridging the gap.
func PlaceRight(width int, left, right string, filler lipgloss.Style) string {
	laneWidth := max(width-lipgloss.Width(left), lipgloss.Width(right))
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		left,
		lipgloss.PlaceHorizontal(
			laneWidth,
			lipgloss.Right,
			right,
			lipgloss.WithWhitespaceStyle(filler),
		),
	)
}

// SplitLines splits a string by newlines, trimming a trailing newline first.
// Returns nil for an empty result so callers can treat it as "no lines".
func SplitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// WrapText wraps value to width using lipgloss.Wrap, returning each line.
// An empty value returns a single-element slice with an empty string.
func WrapText(value string, width int) []string {
	if value == "" {
		return []string{""}
	}
	return strings.Split(lipgloss.Wrap(value, max(width, 1), ""), "\n")
}

// Block is a strings.Builder wrapper that accumulates rendered lines into
// a single string with newline separators.
type Block struct {
	b strings.Builder
}

func (bl *Block) Write(line string) {
	bl.b.WriteString(line)
	bl.b.WriteByte('\n')
}

func (bl *Block) WriteLines(lines []string) {
	for _, l := range lines {
		bl.Write(l)
	}
}

// WriteSection writes lines followed by one blank row. Empty sections skip.
func (bl *Block) WriteSection(lines []string) {
	if len(lines) == 0 {
		return
	}
	bl.WriteLines(lines)
	bl.Write("")
}

func (bl *Block) String() string {
	return bl.b.String()
}
