package manual

import (
	tea "charm.land/bubbletea/v2"
)

func (v Viewer) handleKey(msg tea.KeyPressMsg) (Viewer, tea.Cmd) {
	key := msg.String()
	switch key {
	case "q", "esc", "ctrl+c":
		return v, tea.Quit
	}

	// Digits always jump top-level nav entries.
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		v.jumpTopLevel(int(key[0] - '1'))
		return v, nil
	}

	if v.focus == focusSidebar {
		return v.handleSidebarKey(key)
	}

	// Content focus: scroll only (plus home/end).
	switch key {
	case "tab", "shift+tab":
		v.focus = focusSidebar
		return v, nil
	case "home", "g":
		v.viewport.GotoTop()
	case "end", "G", "shift+g":
		v.viewport.GotoBottom()
	default:
		return v.updateViewport(msg)
	}
	return v, nil
}

func (v Viewer) handleSidebarKey(key string) (Viewer, tea.Cmd) {
	v.refreshRows()
	if len(v.rows) == 0 {
		return v, nil
	}
	cur := v.cursorRow()
	switch key {
	case "tab":
		v.focus = focusContent
		return v, nil
	case "shift+tab":
		v.focus = focusContent
		return v, nil
	case "up", "k", "ctrl+p":
		v.selectRow(stepSelectable(v.rows, cur, -1))
	case "down", "j", "ctrl+n":
		v.selectRow(stepSelectable(v.rows, cur, 1))
	case "right", "l":
		v.expandOrEnter(cur)
	case "left", "h":
		v.collapseOrParent(cur)
	case "enter":
		v.activateRow(cur)
	case "home", "g":
		v.selectRow(stepSelectable(v.rows, -1, 1)) // first selectable
	case "end", "G", "shift+g":
		// step backward from past-end to last selectable
		v.selectRow(stepSelectable(v.rows, len(v.rows), -1))
	}
	return v, nil
}

// selectRow moves the sidebar cursor. Leaf rows also become the body page;
// group rows only move the highlight so the previous leaf stays rendered.
// Separators are ignored.
func (v *Viewer) selectRow(index int) {
	if index < 0 || index >= len(v.rows) {
		return
	}
	row := v.rows[index]
	if row.separator || row.node == nil {
		return
	}
	v.cursorPath = append([]int{}, row.path...)
	if !row.node.IsGroup() {
		v.setLeaf(row.path, true)
	}
}

func (v *Viewer) setLeaf(path []int, resetScroll bool) {
	v.leafPath = append([]int{}, path...)
	v.loadDocsPage(resetScroll)
}

func (v *Viewer) expandOrEnter(index int) {
	if index < 0 || index >= len(v.rows) {
		return
	}
	row := v.rows[index]
	if row.separator || row.node == nil || !row.node.IsGroup() {
		return
	}
	key := pathKey(row.path)
	if !v.expanded[key] {
		if v.expanded == nil {
			v.expanded = map[string]bool{}
		}
		v.expanded[key] = true
		v.refreshRows()
		return
	}
	// already open → move cursor to first selectable child
	next := stepSelectable(v.rows, index, 1)
	if next > index {
		v.selectRow(next)
	}
}
func (v *Viewer) collapseOrParent(index int) {
	if index < 0 || index >= len(v.rows) {
		return
	}
	row := v.rows[index]
	if row.separator || row.node == nil {
		return
	}
	key := pathKey(row.path)
	if row.node.IsGroup() && v.expanded[key] {
		v.expanded[key] = false
		v.refreshRows()
		// Cursor stays on the group; body keeps leafPath.
		return
	}
	if len(row.path) > 1 {
		parent := append([]int{}, row.path[:len(row.path)-1]...)
		v.cursorPath = parent
		v.refreshRows()
	}
}

func (v *Viewer) activateRow(index int) {
	if index < 0 || index >= len(v.rows) {
		return
	}
	row := v.rows[index]
	if row.separator || row.node == nil {
		return
	}
	if row.node.IsGroup() {
		key := pathKey(row.path)
		if v.expanded == nil {
			v.expanded = map[string]bool{}
		}
		v.expanded[key] = !v.expanded[key]
		v.refreshRows()
		return
	}
	v.cursorPath = append([]int{}, row.path...)
	v.setLeaf(row.path, true)
	v.focus = focusContent
}

func (v *Viewer) jumpTopLevel(index int) {
	nav := v.document.Nav
	if index < 0 || index >= len(nav) {
		return
	}
	path := []int{index}
	v.cursorPath = append([]int{}, path...)
	if nav[index].IsGroup() {
		if v.expanded == nil {
			v.expanded = map[string]bool{}
		}
		v.expanded[pathKey(path)] = true
		leaf := firstLeafPath(nav, path)
		if leaf != nil {
			v.cursorPath = append([]int{}, leaf...)
			v.setLeaf(leaf, true)
			v.refreshRows()
			return
		}
	}
	v.setLeaf(path, true)
	v.refreshRows()
}

func (v Viewer) updateViewport(msg tea.Msg) (Viewer, tea.Cmd) {
	var cmd tea.Cmd
	v.viewport, cmd = v.viewport.Update(msg)
	return v, cmd
}

func (v *Viewer) click(x, y int) {
	if x >= v.l.sideW {
		return
	}
	v.refreshRows()
	win := v.windowedRows()
	index := y - SidebarItemsTop
	if index >= 0 && index < len(win) {
		v.focus = focusSidebar
		// Map window-local index back to full rows index.
		v.selectRow(v.windowOffset + index)
	}
}

// onDivider reports whether x is on the vertical sidebar/body junction
// (the single divider column at sideW), with one cell of hit slop.
func (v Viewer) onDivider(x int) bool {
	d := v.l.sideW
	return x >= d-1 && x <= d+1
}

// setSideWidth pins the sidebar to width x (divider x-position) and relayouts.
func (v *Viewer) setSideWidth(x int) {
	maxSide := max(v.width-minBodyW-1, minSideW)
	w := x
	if w < minSideW {
		w = minSideW
	}
	if w > maxSide {
		w = maxSide
	}
	if v.sideWOverride == w && v.l.sideW == w {
		return
	}
	v.sideWOverride = w
	v.recomputeLayout()
}

// recomputeLayout rebuilds the cached layout and viewport content. Called from
// New and on every WindowSizeMsg.
func (v *Viewer) recomputeLayout() {
	offset := v.viewport.YOffset()
	v.refreshRows()
	v.l = v.computeLayout()
	// Keep the viewport constrained to the rendered document width. Its View
	// pads to its configured width with plain cells, so using bodyW here makes
	// those cells consume the right margin before render() can paint it.
	v.viewport.SetWidth(v.l.textW)
	v.viewport.SetHeight(v.l.viewH)
	// Resize keeps the same leaf and scroll offset.
	v.loadDocsPage(false)
	v.viewport.SetYOffset(offset)
}

// loadDocsPage puts only the active leaf page into the body viewport.
// resetScroll is true for navigation so each page opens at its top.
func (v *Viewer) loadDocsPage(resetScroll bool) {
	lines := v.renderActiveContent(v.l.textW)
	v.viewport.SetContentLines(lines)
	if resetScroll {
		v.viewport.GotoTop()
	}
}
