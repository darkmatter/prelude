package motd

import (
	"strings"

	"charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
	glamstyles "charm.land/glamour/v2/styles"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

// Markdown is a React-style, one-component-per-file presentation of a markdown
// prose block. It formats text through glamour so authors can use markdown
// (emphasis, `code`, lists, links, …) in text sections like motd.description.
// The component is stateless and uses the resolved renderer context for
// MOTD-specific theme and layout decisions.
type Markdown struct {
	r renderer
}

// Render formats a prose block through glamour so authors can use markdown
// (emphasis, `code`, lists, links, …) in text sections like motd.description.
// Lines come back wrapped to width with theme styling baked in; the caller
// applies the section fill. Falls back to plain wrapped text if glamour fails.
func (x Markdown) Render(text string, d StyledText, width int) []string {
	tr, err := glamour.NewTermRenderer(
		glamour.WithStyles(x.markdownStyle(d)),
		glamour.WithChromaFormatter("terminal16m"),
		glamour.WithWordWrap(max(width, 1)),
	)
	if err != nil {
		return ui.WrapText(text, width)
	}
	out, err := tr.Render(text)
	if err != nil {
		return ui.WrapText(text, width)
	}
	return trimBlankEdges(strings.Split(strings.TrimRight(out, "\n"), "\n"))
}

// markdownStyle starts from glamour's DarkStyleConfig and overlays the theme
// palette plus the section's StyledText overrides. Every primitive that paints
// text carries an explicit BackgroundColor so glamour never punches holes
// through the MOTD card; margins stay zero — the card owns spacing.
//
// Palette policy:
//   - Document/body text → Fg (or StyledText.Foreground)
//   - Background → StyledText.Background, else block bg when opaque, else none
//   - Headings H1/H3/H5 → Accent2; H2/H4/H6 → Accent
//   - Links/inline code → Accent
//   - Muted/dim → BlockQuote, comments, rules
//   - Semantic Success/Warning/Info/Error → chroma literals / diagnostics
func (x Markdown) markdownStyle(d StyledText) gansi.StyleConfig {
	pal := x.r.st.pal

	fg := string(pal.Fg)
	if d.Foreground != "" {
		fg = d.Foreground
	}

	var bg *string
	switch {
	case d.Background != "":
		bg = new(d.Background)
	case x.r.cfg.Background != "":
		bg = new(colorHex(x.r.st.blockBg))
	}

	accent := new(string(pal.Accent))
	accent2 := new(string(pal.Accent2))
	muted := new(string(pal.Muted))
	dim := new(string(pal.Dim))
	success := new(string(pal.Success))
	warning := new(string(pal.Warning))
	info := new(string(pal.Info))
	errorColor := new(string(pal.Error))
	fgPtr := new(fg)

	base := gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg}
	if d.Bold {
		base.Bold = new(true)
	}
	if d.Italic {
		base.Italic = new(true)
	}
	if d.Faint {
		base.Faint = new(true)
	}

	// Clone DarkStyleConfig so every primitive starts from a complete dark
	// baseline (including Chroma defaults), then re-pin palette + backgrounds.
	style := glamstyles.DarkStyleConfig

	style.Document.StylePrimitive = base
	style.Document.Margin = new(uint(0))
	style.Paragraph.StylePrimitive = base
	style.BlockQuote.StylePrimitive = gansi.StylePrimitive{
		Color:           muted,
		BackgroundColor: bg,
		Italic:          new(true),
	}
	style.BlockQuote.Indent = new(uint(1))
	style.BlockQuote.IndentToken = new("│ ")
	style.List.StyleBlock.StylePrimitive = base
	style.List.LevelIndent = 2

	heading := func(color *string, bold bool) gansi.StyleBlock {
		return gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{
				Color:           color,
				BackgroundColor: bg,
				Bold:            new(bold),
				BlockSuffix:     "\n",
			},
		}
	}
	style.Heading = heading(accent2, true)
	style.H1 = heading(accent2, true)
	style.H2 = heading(accent, true)
	style.H3 = heading(accent2, true)
	style.H4 = heading(accent, true)
	style.H5 = heading(accent2, false)
	style.H6 = heading(accent, false)

	style.Text = base
	style.Strong = gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg, Bold: new(true)}
	style.Emph = gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg, Italic: new(true)}
	style.Strikethrough = gansi.StylePrimitive{Color: muted, BackgroundColor: bg, CrossedOut: new(true)}
	style.HorizontalRule = gansi.StylePrimitive{Color: dim, BackgroundColor: bg, Format: "\n────────\n"}
	style.Item = gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg, BlockPrefix: "• "}
	style.Enumeration = gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg, BlockPrefix: ". "}
	style.Link = gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Underline: new(true)}
	style.LinkText = gansi.StylePrimitive{Color: accent, BackgroundColor: bg}
	// Inline `code` matches the tip cadence: accent + bold.
	style.Code = gansi.StyleBlock{
		StylePrimitive: gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: new(true)},
	}
	style.CodeBlock = gansi.StyleCodeBlock{
		StyleBlock: gansi.StyleBlock{
			StylePrimitive: base,
			Indent:         new(uint(2)),
		},
		Chroma: &gansi.Chroma{
			Text:                gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg},
			Error:               gansi.StylePrimitive{Color: errorColor, BackgroundColor: bg},
			Comment:             gansi.StylePrimitive{Color: muted, BackgroundColor: bg, Italic: new(true)},
			CommentPreproc:      gansi.StylePrimitive{Color: info, BackgroundColor: bg},
			Keyword:             gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: new(true)},
			KeywordReserved:     gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: new(true)},
			KeywordNamespace:    gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			KeywordType:         gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			Operator:            gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			Punctuation:         gansi.StylePrimitive{Color: muted, BackgroundColor: bg},
			Name:                gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg},
			NameBuiltin:         gansi.StylePrimitive{Color: info, BackgroundColor: bg},
			NameTag:             gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			NameAttribute:       gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			NameClass:           gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			NameConstant:        gansi.StylePrimitive{Color: warning, BackgroundColor: bg},
			NameDecorator:       gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			NameException:       gansi.StylePrimitive{Color: errorColor, BackgroundColor: bg},
			NameFunction:        gansi.StylePrimitive{Color: accent2, BackgroundColor: bg},
			NameOther:           gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg},
			Literal:             gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg},
			LiteralNumber:       gansi.StylePrimitive{Color: warning, BackgroundColor: bg},
			LiteralDate:         gansi.StylePrimitive{Color: warning, BackgroundColor: bg},
			LiteralString:       gansi.StylePrimitive{Color: success, BackgroundColor: bg},
			LiteralStringEscape: gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
			GenericDeleted:      gansi.StylePrimitive{Color: errorColor, BackgroundColor: bg},
			GenericEmph:         gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg, Italic: new(true)},
			GenericInserted:     gansi.StylePrimitive{Color: success, BackgroundColor: bg},
			GenericStrong:       gansi.StylePrimitive{Color: fgPtr, BackgroundColor: bg, Bold: new(true)},
			GenericSubheading:   gansi.StylePrimitive{Color: accent2, BackgroundColor: bg, Bold: new(true)},
			Background:          gansi.StylePrimitive{BackgroundColor: bg},
		},
	}

	return style
}

// trimBlankEdges removes leading and trailing rows that carry no visible text
// (glamour block prefixes/suffixes), keeping interior paragraph gaps.
func trimBlankEdges(lines []string) []string {
	start, end := 0, len(lines)
	for start < end && ansi.Strip(lines[start]) == "" {
		start++
	}
	for end > start && ansi.Strip(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}
