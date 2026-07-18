package manual

import (
	tea "charm.land/bubbletea/v2"
)

func (v Viewer) handleKey(msg tea.KeyPressMsg) (Viewer, tea.Cmd) {
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return v, tea.Quit
	case "home", "g":
		v.viewport.GotoTop()
	case "end", "G", "shift+g":
		v.viewport.GotoBottom()
	case "tab":
		v.stepSection(1)
	case "shift+tab":
		v.stepSection(-1)
	default:
		if key := msg.String(); len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			v.jumpSection(int(key[0] - '1'))
			return v, nil
		}
		return v.updateViewport(msg)
	}
	return v, nil
}

func (v Viewer) updateViewport(msg tea.Msg) (Viewer, tea.Cmd) {
	var cmd tea.Cmd
	v.viewport, cmd = v.viewport.Update(msg)
	return v, cmd
}

func (v *Viewer) jumpSection(index int) {
	if index < 0 || index >= len(v.document.Sections) {
		return
	}
	v.viewport.SetYOffset(v.starts[index])
	v.active = index
}

// stepSection jumps to the adjacent section, wrapping at either end, so Tab
// and Shift+Tab cycle through pages without reaching for the digit keys.
func (v *Viewer) stepSection(delta int) {
	count := len(v.document.Sections)
	if count == 0 {
		return
	}
	v.jumpSection(((v.active+delta)%count + count) % count)
}

func (v *Viewer) click(x, y int) {
	if x >= v.l.sideW {
		return
	}
	if index := y - SidebarItemsTop; index >= 0 && index < len(v.document.Sections) {
		v.jumpSection(index)
	}
}

// recomputeLayout rebuilds the cached layout and viewport content. Called from
// New and on every WindowSizeMsg.
func (v *Viewer) recomputeLayout() {
	offset := v.viewport.YOffset()
	v.l = v.computeLayout()
	// Keep the viewport constrained to the rendered document width. Its View
	// pads to its configured width with plain cells, so using bodyW here makes
	// those cells consume the right margin before render() can paint it.
	v.viewport.SetWidth(v.l.textW)
	v.viewport.SetHeight(v.l.viewH)
	lines, starts := v.renderDocument(v.l.textW)
	v.starts = starts
	v.viewport.SetContentLines(lines)
	v.viewport.SetYOffset(offset)
}
