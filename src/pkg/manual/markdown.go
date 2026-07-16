package manual

import (
	"strings"

	"charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
	"github.com/charmbracelet/x/ansi"
)

func (v Viewer) renderMarkdown(source string, textWidth int) []string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(v.markdownStyle()),
		glamour.WithWordWrap(max(textWidth, 1)),
	)
	if err != nil {
		return v.renderPlainMarkdown(source, textWidth)
	}

	output, err := renderer.Render(source)
	if err != nil {
		return v.renderPlainMarkdown(source, textWidth)
	}
	return trimMarkdownBlankEdges(strings.Split(strings.TrimRight(output, "\n"), "\n"))
}

func (v Viewer) renderPlainMarkdown(source string, textWidth int) []string {
	wrapped := strings.Split(ansi.Wordwrap(source, max(textWidth-4, 1), ""), "\n")
	for index, line := range wrapped {
		wrapped[index] = v.styles.onBody(Muted, false).PaddingLeft(2).Render(line)
	}
	return wrapped
}

func (v Viewer) markdownStyle() gansi.StyleConfig {
	palette := v.styles.pal
	foreground := string(palette.Fg)
	background := string(palette.Bg)
	accent := manualStringPtr(string(palette.Accent))
	accent2 := manualStringPtr(string(palette.Accent2))
	muted := manualStringPtr(string(palette.Muted))
	dim := manualStringPtr(string(palette.Dim))

	return gansi.StyleConfig{
		Document: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{
				Color:           manualStringPtr(foreground),
				BackgroundColor: manualStringPtr(background),
			},
			Margin: manualUintPtr(2),
		},
		Paragraph: gansi.StyleBlock{},
		BlockQuote: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: muted, Italic: manualBoolPtr(true)},
			Indent:         manualUintPtr(1),
			IndentToken:    manualStringPtr("│ "),
		},
		List: gansi.StyleList{LevelIndent: 2},
		Heading: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{
				Color:       accent2,
				Bold:        manualBoolPtr(true),
				BlockSuffix: "\n",
			},
		},
		Text:          gansi.StylePrimitive{},
		Strong:        gansi.StylePrimitive{Bold: manualBoolPtr(true)},
		Emph:          gansi.StylePrimitive{Italic: manualBoolPtr(true)},
		Strikethrough: gansi.StylePrimitive{CrossedOut: manualBoolPtr(true)},
		HorizontalRule: gansi.StylePrimitive{
			Color:  dim,
			Format: "\n────────\n",
		},
		Item:        gansi.StylePrimitive{BlockPrefix: "• "},
		Enumeration: gansi.StylePrimitive{BlockPrefix: ". "},
		Link: gansi.StylePrimitive{
			Color:     accent,
			Underline: manualBoolPtr(true),
		},
		LinkText: gansi.StylePrimitive{Color: accent},
		Code: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: accent, Bold: manualBoolPtr(true)},
		},
		CodeBlock: gansi.StyleCodeBlock{
			StyleBlock: gansi.StyleBlock{
				StylePrimitive: gansi.StylePrimitive{Color: muted},
				Indent:         manualUintPtr(2),
			},
		},
	}
}

func trimMarkdownBlankEdges(lines []string) []string {
	blank := func(line string) bool { return strings.TrimSpace(ansi.Strip(line)) == "" }
	start, end := 0, len(lines)
	for start < end && blank(lines[start]) {
		start++
	}
	for end > start && blank(lines[end-1]) {
		end--
	}
	return lines[start:end]
}

func manualStringPtr(value string) *string { return &value }
func manualBoolPtr(value bool) *bool       { return &value }
func manualUintPtr(value uint) *uint       { return &value }
