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
	side := lipgloss.Width("CONTENTS")
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
	lines = append(lines, "")
	for _, section := range v.document.Sections {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		starts = append(starts, len(lines))
		if section.Markdown != "" {
			lines = append(lines, v.renderMarkdown(section.Markdown, textWidth)...)
			continue
		}
		lines = append(lines, v.styles.onBody(Accent, true).PaddingLeft(2).Render(strings.ToUpper(section.Title)))
		for _, block := range section.Blocks {
			lines = append(lines, v.renderBlock(block, textWidth)...)
			if block.BlankAfter {
				lines = append(lines, "")
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
	if block.Wrap {
		role, bold := block.Spans[0].Role, block.Spans[0].Bold
		wrapped := strings.Split(ansi.Wordwrap(plain.String(), max(textWidth-block.Indent, 16), ""), "\n")
		lines := make([]string, len(wrapped))
		for index, line := range wrapped {
			lines[index] = v.styles.onBody(role, bold).PaddingLeft(block.Indent).Render(line)
		}
		return lines
	}

	var line strings.Builder
	line.WriteString(v.styles.bodySpace.PaddingLeft(block.Indent).Render(""))
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
		sections: v.document.Sections,
		active:   v.active,
		styles:   v.styles,
		width:    l.sideW,
		height:   l.viewH,
	}
	sideLines := strings.Split(sb.View(), "\n")

	// Body column — rendered and scrolled by the Bubbles viewport.
	padBody := func(content string) string {
		return v.styles.bodySpace.Inline(true).Width(l.bodyW).MaxWidth(l.bodyW).Render(content)
	}
	bodyLines := strings.Split(v.viewport.View(), "\n")

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
		rows = append(rows, sideLines[row]+divider.Render(junction)+padBody(body))
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
		styles:    v.styles,
	}
	rows = append(rows, stBar.View())

	return strings.Join(rows, "\n")
}
