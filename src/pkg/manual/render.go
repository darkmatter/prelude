package manual

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type layout struct {
	sideW int
	bodyW int
	textW int
	viewH int
}

const (
	minSideW     = 12
	minBodyW     = 20
	scrollBarW   = 1 // subtle right-edge overflow indicator
)

func (v Viewer) computeLayout() layout {
	label := v.document.SidebarLabel()
	side := lipgloss.Width(label)
	for _, row := range v.rows {
		if row.separator || row.node == nil {
			continue
		}
		// indent + digit + branch/title (+ group mark)
		w := indentWidth + digitWidth + lipgloss.Width(row.branch) + lipgloss.Width(row.node.Title)
		if row.depth == 0 && row.node.IsGroup() {
			w += 2 // v / >
		}
		side = max(side, w)
	}
	side += 2
	// User drag override wins over auto-fit, then clamp so body stays usable.
	if v.sideWOverride > 0 {
		side = v.sideWOverride
	}
	maxSide := max(v.width-minBodyW-1, minSideW)
	if side < minSideW {
		side = minSideW
	}
	if side > maxSide {
		side = maxSide
	}
	body := max(v.width-side-1, minBodyW)
	// textW is content width inside the body: leave room for scrollbar column
	// and a little breathing room before the 96-col soft cap.
	textW := min(max(body-scrollBarW-2, 16), 96)
	return layout{
		sideW: side,
		bodyW: body,
		textW: textW,
		viewH: max(v.height-2, 1),
	}
}

// renderActiveContent paints the body viewport for the active leaf.
// Each page is discrete: only the active leaf's Markdown is shown.
func (v Viewer) renderActiveContent(textWidth int) []string {
	leaf := v.activeLeaf()
	if leaf == nil {
		return []string{v.styles.blankLine(max(textWidth, 24))}
	}
	if leaf.RootReadme {
		return v.renderRootReadme(leaf, textWidth)
	}
	return v.renderLeaf(leaf, textWidth)
}

// renderLeaf paints one docs page's Markdown into viewport lines.
func (v Viewer) renderLeaf(leaf *NavNode, textWidth int) []string {
	textWidth = max(textWidth, 24)
	// Seed with a painted blank so the viewport never holds a raw empty string
	// (those punch holes through to the terminal default background).
	lines := []string{v.styles.blankLine(textWidth)}
	if leaf.Markdown != "" {
		for _, line := range v.renderMarkdown(leaf.Markdown, textWidth) {
			lines = append(lines, v.styles.fillLine(line, textWidth))
		}
	}
	return lines
}

// render composes the full screen from the cached layout and three
// sub-model views: sidebar column, document body column, and status bar row.
// The sidebar and body are zipped row-by-row with a vertical border junction
// between them. Focus is shown by painting the focused pane's top border in
// accent — border style stays Normal on both sides.
func (v Viewer) render() string {
	l := v.l
	scroll := v.viewport.YOffset()
	maxScroll := max(0, v.viewport.TotalLineCount()-v.viewport.Height())

	border := lipgloss.NormalBorder()
	navFocus := v.focus == focusSidebar
	bodyFocus := v.focus == focusContent

	// Top border: accent on the focused pane, idle border elsewhere.
	topNav := v.styles.frame
	if navFocus {
		topNav = v.styles.frameAccent
	}
	topBody := v.styles.divider
	if bodyFocus {
		topBody = v.styles.topAccent
	}
	divStyle := v.styles.divider

	// Sidebar column — windowed to viewH so the cursor never leaves the pane.
	var sideRows []treeRow
	cursorLocal := 0
	cur := selectableRowIndex(v.rows, v.cursorPath)
	sideRows, _ = windowRows(v.rows, cur, max(l.viewH-itemsTop, 1))
	ck := pathKey(v.cursorPath)
	for i, r := range sideRows {
		if !r.separator && pathKey(r.path) == ck {
			cursorLocal = i
			break
		}
	}
	sb := sidebarView{
		label:     v.document.SidebarLabel(),
		rows:      sideRows,
		cursor:    cursorLocal,
		focusSide: v.focus == focusSidebar,
		styles:    v.styles,
		width:     l.sideW,
		height:    l.viewH,
	}
	sideLines := strings.Split(sb.View(), "\n")

	// Body column — viewport content is sealed to textW; pad to bodyW-1 and
	// append a 1-col scrollbar gutter (track + thumb when content overflows).
	bodyLines := strings.Split(v.viewport.View(), "\n")
	contentW := max(l.bodyW-scrollBarW, 1)
	padBody := func(content string) string {
		return v.styles.fillLine(content, contentW)
	}
	totalLines := v.viewport.TotalLineCount()
	bar := v.scrollBarColumn(l.viewH, scroll, maxScroll, totalLines)

	// Top border row — same glyphs both panes; accent color marks focus.
	topBorder := topNav.Render(strings.Repeat(border.Top, l.sideW)) +
		topBody.Render(border.MiddleTop+strings.Repeat(border.Top, l.bodyW))

	// Content rows: zip sidebar + junction + body + scrollbar.
	// Focus accent is only the window top border — the sidebar header rule
	// under PAGES stays idle so it doesn't double-highlight.
	rows := make([]string, 0, l.viewH+2)
	rows = append(rows, topBorder)
	for row := range l.viewH {
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
		sbCell := bar[row]
		rows = append(rows, side+divStyle.Render(junction)+padBody(body)+sbCell)
	}

	// Status bar — rendered by the statusBarView sub-model.
	section := ""
	if leaf := v.activeLeaf(); leaf != nil {
		section = leaf.Title
	}
	jumpCount := min(len(v.document.Nav), 9)
	stBar := statusBarView{
		width:     v.width,
		scroll:    scroll,
		maxScroll: maxScroll,
		section:   section,
		jumpCount: jumpCount,
		mode:      v.document.ModeLabel(),
		styles:    v.styles,
	}
	rows = append(rows, stBar.View())

	return strings.Join(rows, "\n")
}

// scrollBarColumn builds a 1-cell-wide vertical scrollbar for the body.
// When content fits, the gutter is blank body fill (no visual noise).
// When overflow exists: dim track │ with a muted thumb ┃ sized to the viewport.
func (v Viewer) scrollBarColumn(height, yOffset, maxScroll, totalLines int) []string {
	out := make([]string, height)
	blank := v.styles.bodySpace.Width(1).Render(" ")
	if height <= 0 {
		return out
	}
	if maxScroll <= 0 || totalLines <= height {
		for i := range out {
			out[i] = blank
		}
		return out
	}

	// Thumb covers the visible fraction of the document, at least 1 row.
	thumb := max(1, (height*height)/max(totalLines, 1))
	if thumb > height {
		thumb = height
	}
	// Map yOffset ∈ [0, maxScroll] → thumb start ∈ [0, height-thumb].
	start := 0
	if maxScroll > 0 && height > thumb {
		start = (yOffset * (height - thumb)) / maxScroll
	}
	end := start + thumb

	track := v.styles.scrollTrack.Render("│")
	thumbCell := v.styles.scrollThumb.Render("│")
	for i := range out {
		if i >= start && i < end {
			out[i] = thumbCell
		} else {
			out[i] = track
		}
	}
	return out
}

