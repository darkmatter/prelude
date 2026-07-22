package manual

import (
	"strings"

	"charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
	glamstyles "charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"
)

func (v Viewer) renderMarkdown(source string, textWidth int) []string {
	return v.renderMarkdownBody(source, textWidth)
}

// renderMarkdownBody renders Markdown through glamour.
func (v Viewer) renderMarkdownBody(source string, textWidth int) []string {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(v.markdownStyle()),
		glamour.WithChromaFormatter("terminal16m"),
		glamour.WithWordWrap(max(textWidth, 1)),
	)
	if err != nil {
		return v.renderPlainMarkdown(source, textWidth)
	}

	output, err := renderer.Render(source)
	if err != nil {
		return v.renderPlainMarkdown(source, textWidth)
	}
	return strings.Split(strings.TrimRight(output, "\n"), "\n")
}

func (v Viewer) renderPlainMarkdown(source string, textWidth int) []string {
	wrapped := strings.Split(ansi.Wordwrap(source, max(textWidth-4, 1), ""), "\n")
	indent := v.styles.indent(2)
	for index, line := range wrapped {
		wrapped[index] = indent + v.styles.onBody(Muted, false).Render(line)
	}
	return wrapped
}

func (v Viewer) markdownStyle() gansi.StyleConfig {
	palette := v.styles.pal
	accent := manualStringPtr(string(palette.Accent))
	accent2 := manualStringPtr(string(palette.Accent2))
	muted := manualStringPtr(string(palette.Muted))
	dim := manualStringPtr(string(palette.Dim))
	success := manualStringPtr(string(palette.Success))
	warning := manualStringPtr(string(palette.Warning))
	info := manualStringPtr(string(palette.Info))
	errorColor := manualStringPtr(string(palette.Error))
	bg := manualStringPtr(string(palette.Bg))
	fg := manualStringPtr(string(palette.Fg))

	style := glamstyles.DarkStyleConfig
	base := gansi.StylePrimitive{Color: fg, BackgroundColor: bg}

	style.Document.StylePrimitive = base
	style.Document.Margin = manualUintPtr(0)
	// Indent + Margin: Indent alone only pads the first line in some glamour
	// paths; Margin keeps wrapped continuation lines aligned (chip/prose wrap).
	style.Paragraph.StylePrimitive = base
	style.Paragraph.Indent = manualUintPtr(0)
	style.Paragraph.Margin = manualUintPtr(2)
	style.BlockQuote.StylePrimitive.Color = muted
	style.BlockQuote.StylePrimitive.BackgroundColor = bg
	style.BlockQuote.StylePrimitive.Italic = manualBoolPtr(true)
	style.BlockQuote.Indent = manualUintPtr(1)
	style.BlockQuote.IndentToken = manualStringPtr("  │ ")
	style.List.StyleBlock.StylePrimitive = base
	style.List.StyleBlock.Indent = manualUintPtr(2)
	style.List.StyleBlock.Margin = manualUintPtr(2)
	style.List.LevelIndent = 2
	style.Heading.StylePrimitive.Color = accent2
	style.Heading.StylePrimitive.BackgroundColor = bg
	style.Heading.StylePrimitive.Bold = manualBoolPtr(true)
	style.Heading.StylePrimitive.BlockSuffix = "\n"
	style.Heading.Indent = manualUintPtr(2)

	style.H1.StylePrimitive.Color = accent2
	style.H1.StylePrimitive.BackgroundColor = bg
	style.H1.StylePrimitive.Bold = manualBoolPtr(true)
	style.H1.StylePrimitive.BlockSuffix = "\n"
	style.H2.StylePrimitive.Color = accent
	style.H2.StylePrimitive.BackgroundColor = bg
	style.H2.StylePrimitive.Bold = manualBoolPtr(true)
	style.H2.StylePrimitive.BlockSuffix = "\n"
	style.H3.StylePrimitive.Color = accent2
	style.H3.StylePrimitive.BackgroundColor = bg
	style.H3.StylePrimitive.Bold = manualBoolPtr(true)
	style.H3.StylePrimitive.BlockSuffix = "\n"
	style.H4.StylePrimitive.Color = accent
	style.H4.StylePrimitive.BackgroundColor = bg
	style.H4.StylePrimitive.Bold = manualBoolPtr(true)
	style.H5.StylePrimitive.Color = accent2
	style.H5.StylePrimitive.BackgroundColor = bg
	style.H5.StylePrimitive.Bold = manualBoolPtr(false)
	style.H6.StylePrimitive.Color = accent
	style.H6.StylePrimitive.BackgroundColor = bg
	style.H6.StylePrimitive.Bold = manualBoolPtr(false)

	style.Text = base
	style.Strong = gansi.StylePrimitive{Color: fg, BackgroundColor: bg, Bold: manualBoolPtr(true)}
	style.Emph = gansi.StylePrimitive{Color: fg, BackgroundColor: bg, Italic: manualBoolPtr(true)}
	style.Strikethrough = gansi.StylePrimitive{Color: muted, BackgroundColor: bg, CrossedOut: manualBoolPtr(true)}
	style.HorizontalRule = gansi.StylePrimitive{Color: dim, BackgroundColor: bg, Format: "\n────────\n"}
	style.Item = gansi.StylePrimitive{Color: fg, BackgroundColor: bg, BlockPrefix: "• "}
	style.Enumeration = gansi.StylePrimitive{Color: fg, BackgroundColor: bg, BlockPrefix: ". "}
	style.Link = gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Underline: manualBoolPtr(true)}
	style.LinkText = gansi.StylePrimitive{Color: accent, BackgroundColor: bg}
	style.Code = gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: manualBoolPtr(true)}}
	style.CodeBlock = gansi.StyleCodeBlock{
		StyleBlock: gansi.StyleBlock{
			StylePrimitive: base,
			Indent:         manualUintPtr(4),
		},
		Chroma: &gansi.Chroma{
			Text:                gansi.StylePrimitive{Color: fg, BackgroundColor: bg},
			Error:               gansi.StylePrimitive{Color: errorColor, BackgroundColor: bg},
			Comment:             gansi.StylePrimitive{Color: muted, BackgroundColor: bg, Italic: manualBoolPtr(true)},
			CommentPreproc:      gansi.StylePrimitive{Color: info, BackgroundColor: bg},
			Keyword:             gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: manualBoolPtr(true)},
			KeywordReserved:     gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: manualBoolPtr(true)},
			KeywordNamespace:    gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			KeywordType:         gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			Operator:            gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			Punctuation:         gansi.StylePrimitive{Color: muted, BackgroundColor: bg},
			Name:                gansi.StylePrimitive{Color: fg, BackgroundColor: bg},
			NameBuiltin:         gansi.StylePrimitive{Color: info, BackgroundColor: bg},
			NameTag:             gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			NameAttribute:       gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			NameClass:           gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			NameConstant:        gansi.StylePrimitive{Color: warning, BackgroundColor: bg},
			NameDecorator:       gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			NameException:       gansi.StylePrimitive{Color: errorColor, BackgroundColor: bg},
			NameFunction:        gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			NameOther:           gansi.StylePrimitive{Color: fg, BackgroundColor: bg},
			Literal:             gansi.StylePrimitive{Color: fg, BackgroundColor: bg},
			LiteralNumber:       gansi.StylePrimitive{Color: warning, BackgroundColor: bg},
			LiteralDate:         gansi.StylePrimitive{Color: warning, BackgroundColor: bg},
			LiteralString:       gansi.StylePrimitive{Color: success, BackgroundColor: bg},
			LiteralStringEscape: gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			GenericDeleted:      gansi.StylePrimitive{Color: errorColor, BackgroundColor: bg},
			GenericEmph:         gansi.StylePrimitive{Color: fg, BackgroundColor: bg, Italic: manualBoolPtr(true)},
			GenericInserted:     gansi.StylePrimitive{Color: success, BackgroundColor: bg},
			GenericStrong:       gansi.StylePrimitive{Color: fg, BackgroundColor: bg, Bold: manualBoolPtr(true)},
			GenericSubheading:   gansi.StylePrimitive{Color: accent2, BackgroundColor: bg, Bold: manualBoolPtr(true)},
			Background:          gansi.StylePrimitive{BackgroundColor: bg},
		},
	}

	return style
}

func manualStringPtr(value string) *string { return &value }
func manualBoolPtr(value bool) *bool       { return &value }
func manualUintPtr(value uint) *uint       { return &value }
