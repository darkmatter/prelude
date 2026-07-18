package manual

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type layout struct {
	sideW int
	bodyW int
	textW int
	viewH int
}

func (v Viewer) computeLayout() layout {
	label := v.document.SidebarLabel()
	side := lipgloss.Width(label)
	for _, section := range v.document.Sections {
		side = max(side, lipgloss.Width(section.Title)+2)
	}
	side += 4
	body := max(v.width-side-1, 20)
	return layout{
		sideW: side,
		bodyW: body,
		textW: min(body-2, 96),
		viewH: max(v.height-2, 1),
	}
}

func (v Viewer) renderDocument(textWidth int) (lines []string, starts []int) {
	textWidth = max(textWidth, 24)
	isBlank := func(line string) bool {
		return strings.TrimSpace(ansi.Strip(line)) == ""
	}
	// Seed with a painted blank so the viewport never holds a raw empty string
	// (those punch holes through to the terminal default background).
	lines = append(lines, v.styles.blankLine(textWidth))
	for _, section := range v.document.Sections {
		if len(lines) > 0 && !isBlank(lines[len(lines)-1]) {
			lines = append(lines, v.styles.blankLine(textWidth))
		}
		starts = append(starts, len(lines))
		if section.Markdown != "" {
			for _, line := range v.renderMarkdown(section.Markdown, textWidth) {
				lines = append(lines, v.styles.fillLine(line, textWidth))
			}
			continue
		}
		// Structured manuals: paint heading + blocks, then seal each row to
		// textWidth so trailing cells keep the body fill.
		title := v.styles.indent(2) + v.styles.onBody(Accent, true).Render(strings.ToUpper(section.Title))
		lines = append(lines, v.styles.fillLine(title, textWidth))
		for _, block := range section.Blocks {
			for _, line := range v.renderBlock(block, textWidth) {
				lines = append(lines, v.styles.fillLine(line, textWidth))
			}
			if block.BlankAfter {
				lines = append(lines, v.styles.blankLine(textWidth))
			}
		}
	}
	return lines, starts
}

func (v Viewer) renderBlock(block Block, textWidth int) []string {
	if len(block.Spans) == 0 {
		return nil
	}
	plain := strings.Builder{}
	for _, span := range block.Spans {
		plain.WriteString(span.Text)
	}
	if plain.Len() == 0 {
		return nil
	}
	// Indent with a filled run rather than Style.PaddingLeft: lipgloss padding
	// inserts a reset between pad and content, which is fine when both share a
	// bg, but an explicit fill indent matches the rest of the codebase.
	pad := v.styles.indent(block.Indent)
	if block.Wrap {
		role, bold := block.Spans[0].Role, block.Spans[0].Bold
		wrapped := strings.Split(ansi.Wordwrap(plain.String(), max(textWidth-block.Indent, 16), ""), "\n")
		lines := make([]string, len(wrapped))
		for index, line := range wrapped {
			lines[index] = pad + v.styles.onBody(role, bold).Render(line)
		}
		return lines
	}

	var line strings.Builder
	line.WriteString(pad)
	for _, span := range block.Spans {
		line.WriteString(v.styles.onBody(span.Role, span.Bold).Render(span.Text))
	}
	return []string{line.String()}
}

// render composes the full screen from the cached layout and three
// sub-model views: sidebar column, document body column, and status bar row.
// The sidebar and body are zipped row-by-row with a vertical border junction
// between them.
func (v Viewer) render() string {
	l := v.l
	scroll := v.viewport.YOffset()
	maxScroll := max(0, v.viewport.TotalLineCount()-v.viewport.Height())

	border := lipgloss.NormalBorder()
	divider := v.styles.onBody(Foreground, false).Foreground(lipgloss.Color(string(v.styles.pal.Border)))

	// Sidebar column — rendered by the sidebarView sub-model.
	sb := sidebarView{
		label:    v.document.SidebarLabel(),
		kind:     v.document.Kind,
		sections: v.document.Sections,
		active:   v.active,
		styles:   v.styles,
		width:    l.sideW,
		height:   l.viewH,
	}
	sideLines := strings.Split(sb.View(), "\n")

	// Body column — viewport content is already sealed to textWidth; pad out to
	// bodyW with lipgloss whitespace styling (not a hand-rolled Width pad).
	bodyLines := strings.Split(v.viewport.View(), "\n")
	padBody := func(content string) string {
		return v.styles.fillLine(content, l.bodyW)
	}

	// Top border row.
	topBorder := v.styles.frame.Render(strings.Repeat(border.Top, l.sideW)) +
		divider.Render(border.MiddleTop+strings.Repeat(border.Top, l.bodyW))

	// Content rows: zip sidebar + junction + body.
	rows := make([]string, 0, l.viewH+2)
	rows = append(rows, topBorder)
	for row := 0; row < l.viewH; row++ {
		junction := border.Right
		if row == 2 {
			junction = border.MiddleRight
		}
		body := ""
		if row < len(bodyLines) {
			body = bodyLines[row]
		}
		side := ""
		if row < len(sideLines) {
			side = sideLines[row]
		} else {
			side = v.styles.surfaceSpace.Width(l.sideW).Render("")
		}
		rows = append(rows, side+divider.Render(junction)+padBody(body))
	}

	// Status bar — rendered by the statusBarView sub-model.
	section := ""
	if count := len(v.document.Sections); count > 0 {
		section = v.document.Sections[min(v.active, count-1)].Title
	}
	stBar := statusBarView{
		width:     v.width,
		scroll:    scroll,
		maxScroll: maxScroll,
		section:   section,
		jumpCount: min(len(v.document.Sections), 9),
		kind:      v.document.Kind,
		mode:      v.document.ModeLabel(),
		styles:    v.styles,
	}
	rows = append(rows, stBar.View())

	return strings.Join(rows, "\n")
}
