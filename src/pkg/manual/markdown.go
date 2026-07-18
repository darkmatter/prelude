package manual

import (
	"strings"

	"charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	gtext "github.com/yuin/goldmark/text"

	"prelude/pkg/ui"
)

// renderMarkdown renders one page. Top-level `##` headings become labeled
// fading rules — the same header treatment as the code-block element — so
// sections separate visually; everything between them renders through glamour.
func (v Viewer) renderMarkdown(source string, textWidth int) []string {
	var lines []string
	for _, segment := range splitAtH2(source) {
		if segment.title != "" {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, v.sectionRule(segment.title, textWidth), "")
		}
		if strings.TrimSpace(segment.body) == "" {
			continue
		}
		lines = append(lines, v.renderMarkdownBody(segment.body, textWidth)...)
	}
	return lines
}

// markdownSegment is one H2-delimited slice of a page; the leading segment
// (before any H2) carries an empty title.
type markdownSegment struct {
	title string
	body  string
}

// splitAtH2 splits markdown at top-level `##` headings. Parsing with goldmark
// keeps fenced code blocks containing "## …" lines intact.
func splitAtH2(source string) []markdownSegment {
	contents := []byte(source)
	root := goldmark.DefaultParser().Parse(gtext.NewReader(contents))
	segments := []markdownSegment{{}}
	cursor := 0
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		heading, ok := node.(*ast.Heading)
		if !ok || heading.Level != 2 || heading.Lines().Len() == 0 {
			continue
		}
		title := strings.TrimSpace(string(heading.Text(contents)))
		if title == "" {
			continue
		}
		segment := heading.Lines().At(0)
		// The heading segment covers the text only; extend to the "## " line
		// start so the marker never leaks into the previous body.
		start := segment.Start
		for start > 0 && contents[start-1] != '\n' {
			start--
		}
		segments[len(segments)-1].body = source[cursor:start]
		segments = append(segments, markdownSegment{title: title})
		cursor = segment.Stop
	}
	segments[len(segments)-1].body = source[cursor:]
	return segments
}

// sectionRule renders an H2 title inset in a fading horizontal rule, mirroring
// the code-block header so both section markers share one visual language.
// The context is non-transparent on the body surface: every dash and label
// cell must carry the body background (the viewport is a solid surface).
func (v Viewer) sectionRule(title string, textWidth int) string {
	rule := ui.FadingRule{
		Context: ui.NewContext(v.styles.pal, v.styles.bg, false),
		Width:   max(textWidth-2, 1),
		Frame:   lipgloss.Color(string(v.styles.pal.AccentBorder)),
		Fade:    true,
	}
	return v.styles.indent(2) + rule.Render(title)
}

// renderMarkdownBody renders one H2-free chunk through glamour.
func (v Viewer) renderMarkdownBody(source string, textWidth int) []string {
	// Leave a 2-cell indent we paint ourselves with the body fill. Glamour's
	// Document.Margin inserts unstyled spaces that punch holes through to the
	// terminal default background — never use it when the viewer owns the bg.
	innerWidth := max(textWidth-2, 1)
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(v.markdownStyle()),
		glamour.WithWordWrap(innerWidth),
	)
	if err != nil {
		return v.renderPlainMarkdown(source, textWidth)
	}

	output, err := renderer.Render(source)
	if err != nil {
		return v.renderPlainMarkdown(source, textWidth)
	}
	raw := trimMarkdownBlankEdges(strings.Split(strings.TrimRight(output, "\n"), "\n"))
	indent := v.styles.indent(2)
	lines := make([]string, len(raw))
	for i, line := range raw {
		// Prefix every row with filled indent so the left margin carries bg.
		lines[i] = indent + line
	}
	return lines
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
	foreground := string(palette.Fg)
	background := string(palette.Bg)
	accent := manualStringPtr(string(palette.Accent))
	accent2 := manualStringPtr(string(palette.Accent2))
	muted := manualStringPtr(string(palette.Muted))
	dim := manualStringPtr(string(palette.Dim))
	bg := manualStringPtr(background)
	fg := manualStringPtr(foreground)

	// Every primitive carries BackgroundColor so glamour spans do not reset to
	// the terminal default between style changes (the classic "hole" look).
	base := gansi.StylePrimitive{Color: fg, BackgroundColor: bg}

	return gansi.StyleConfig{
		Document: gansi.StyleBlock{
			StylePrimitive: base,
			// Margin must stay 0 — glamour margin is unstyled whitespace.
			Margin: manualUintPtr(0),
		},
		Paragraph: gansi.StyleBlock{StylePrimitive: base},
		BlockQuote: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: muted, BackgroundColor: bg, Italic: manualBoolPtr(true)},
			Indent:         manualUintPtr(1),
			IndentToken:    manualStringPtr("│ "),
		},
		List: gansi.StyleList{LevelIndent: 2},
		Heading: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{
				Color:           accent2,
				BackgroundColor: bg,
				Bold:            manualBoolPtr(true),
				BlockSuffix:     "\n",
			},
		},
		H1: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{
			Color: accent2, BackgroundColor: bg, Bold: manualBoolPtr(true), BlockSuffix: "\n",
		}},
		H2: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{
			Color: accent2, BackgroundColor: bg, Bold: manualBoolPtr(true), BlockSuffix: "\n",
		}},
		H3: gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{
			Color: accent, BackgroundColor: bg, Bold: manualBoolPtr(true), BlockSuffix: "\n",
		}},
		H4:            gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: manualBoolPtr(true)}},
		H5:            gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: muted, BackgroundColor: bg, Bold: manualBoolPtr(true)}},
		H6:            gansi.StyleBlock{StylePrimitive: gansi.StylePrimitive{Color: dim, BackgroundColor: bg, Bold: manualBoolPtr(true)}},
		Text:          base,
		Strong:        gansi.StylePrimitive{Color: fg, BackgroundColor: bg, Bold: manualBoolPtr(true)},
		Emph:          gansi.StylePrimitive{Color: fg, BackgroundColor: bg, Italic: manualBoolPtr(true)},
		Strikethrough: gansi.StylePrimitive{Color: muted, BackgroundColor: bg, CrossedOut: manualBoolPtr(true)},
		HorizontalRule: gansi.StylePrimitive{
			Color: dim, BackgroundColor: bg, Format: "\n────────\n",
		},
		Item:        gansi.StylePrimitive{Color: fg, BackgroundColor: bg, BlockPrefix: "• "},
		Enumeration: gansi.StylePrimitive{Color: fg, BackgroundColor: bg, BlockPrefix: ". "},
		Link: gansi.StylePrimitive{
			Color: accent, BackgroundColor: bg, Underline: manualBoolPtr(true),
		},
		LinkText: gansi.StylePrimitive{Color: accent, BackgroundColor: bg},
		Code: gansi.StyleBlock{
			StylePrimitive: gansi.StylePrimitive{Color: accent, BackgroundColor: bg, Bold: manualBoolPtr(true)},
		},
		CodeBlock: gansi.StyleCodeBlock{
			StyleBlock: gansi.StyleBlock{
				StylePrimitive: gansi.StylePrimitive{Color: muted, BackgroundColor: bg},
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
