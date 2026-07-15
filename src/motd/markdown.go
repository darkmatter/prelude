package main

import (
	"strings"

	"charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
	"github.com/charmbracelet/x/ansi"
)

// renderMarkdownBlock formats a prose block through glamour so authors can use
// markdown (emphasis, `code`, lists, links, …) in text sections like
// motd.description. Lines come back wrapped to width with theme styling baked
// in; the caller applies the section fill. Falls back to plain wrapped text if
// glamour fails.
func (r renderer) renderMarkdownBlock(text string, d StyledText, width int) []string {
	tr, err := glamour.NewTermRenderer(
		glamour.WithStyles(r.markdownStyle(d)),
		glamour.WithWordWrap(max(width, 1)),
	)
	if err != nil {
		return wrapText(text, width)
	}
	out, err := tr.Render(text)
	if err != nil {
		return wrapText(text, width)
	}
	return trimBlankEdges(strings.Split(strings.TrimRight(out, "\n"), "\n"))
}

// markdownStyle derives a glamour stylesheet from the theme palette plus the
// section's StyledText overrides. Margins stay zero — the card owns spacing.
func (r renderer) markdownStyle(d StyledText) gansi.StyleConfig {
	pal := r.st.pal

	fg := string(pal.Fg)
	if d.Foreground != "" {
		fg = d.Foreground
	}
	doc := gansi.StylePrimitive{Color: strPtr(fg)}
	switch {
	case d.Background != "":
		doc.BackgroundColor = strPtr(d.Background)
	case r.cfg.Background != "":
		doc.BackgroundColor = strPtr(colorHex(r.st.blockBg))
	}
	if d.Bold {
		doc.Bold = boolPtr(true)
	}
	if d.Italic {
		doc.Italic = boolPtr(true)
	}
	if d.Faint {
		doc.Faint = boolPtr(true)
	}

	accent := strPtr(string(pal.Accent))
	accent2 := strPtr(string(pal.Accent2))
	muted := strPtr(string(pal.Muted))
	dim := strPtr(string(pal.Dim))

	return gansi.StyleConfig{
		Document: gansi.StyleBlock{
			StylePrimitive: doc,
			Margin:         uintPtr(0),
		},
		Paragraph: gansi.StyleBlock{},
		BlockQuote: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: muted, Italic: boolPtr(true)},
			Indent:         uintPtr(1),
			IndentToken:    strPtr("│ "),
		},
		List: gansi.StyleList{LevelIndent: 2},
		Heading: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{
				Color:       accent2,
				Bold:        boolPtr(true),
				BlockSuffix: "\n",
			},
		},
		Text:          gansi.StylePrimitive{},
		Strong:        gansi.StylePrimitive{Bold: boolPtr(true)},
		Emph:          gansi.StylePrimitive{Italic: boolPtr(true)},
		Strikethrough: gansi.StylePrimitive{CrossedOut: boolPtr(true)},
		HorizontalRule: gansi.StylePrimitive{
			Color:  dim,
			Format: "\n────────\n",
		},
		Item:        gansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: gansi.StylePrimitive{BlockPrefix: ". "},
		Link: gansi.StylePrimitive{
			Color:     accent,
			Underline: boolPtr(true),
		},
		LinkText: gansi.StylePrimitive{Color: accent},
		// Inline `code` matches the tip cadence: accent + bold.
		Code: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: accent, Bold: boolPtr(true)},
		},
		CodeBlock: gansi.StyleCodeBlock{
			StyleBlock: gansi.StyleBlock{
				StylePrimitive: gansi.StylePrimitive{Color: muted},
				Indent:         uintPtr(2),
			},
		},
	}
}

// trimBlankEdges removes leading and trailing rows that carry no visible text
// (glamour block prefixes/suffixes), keeping interior paragraph gaps.
func trimBlankEdges(lines []string) []string {
	blank := func(s string) bool { return strings.TrimSpace(ansi.Strip(s)) == "" }
	start, end := 0, len(lines)
	for start < end && blank(lines[start]) {
		start++
	}
	for end > start && blank(lines[end-1]) {
		end--
	}
	return lines[start:end]
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }
func uintPtr(u uint) *uint    { return &u }
