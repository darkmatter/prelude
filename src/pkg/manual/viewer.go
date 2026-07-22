// Package manual renders and navigates the Prelude docs viewer.
package manual

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"prelude/pkg/shared"
)

// focusTarget is which pane receives navigation keys.
type focusTarget uint8

const (
	focusContent focusTarget = iota
	focusSidebar
)

// Viewer owns docs state, navigation, layout, chrome, and presentation.
type Viewer struct {
	document Document
	styles   styles

	width    int
	height   int
	viewport viewport.Model

	// cursorPath is the sidebar highlight (may be a group).
	// leafPath is the leaf whose Markdown is shown in the body; only updated
	// when a leaf is selected/activated so hovering groups never blanks the page.
	cursorPath   []int
	leafPath     []int
	expanded     map[string]bool
	focus        focusTarget
	rows         []treeRow // full visible tree (not windowed)
	windowOffset int       // first rows index shown in the sidebar pane

	// sideWOverride is a user-chosen sidebar width from dragging the divider.
	// 0 means auto-size from labels. dragging is true while the left button is
	// held on the vertical junction.
	sideWOverride int
	dragging      bool

	// l is recomputed on every WindowSizeMsg. The docs viewer loads one page
	// at a time; starts stays nil.
	l layout
}

// New constructs a viewer with the default terminal dimensions.
func New(document Document, palette shared.Palette) Viewer {
	viewerStyles := newStyles(palette)
	body := viewport.New(
		viewport.WithWidth(80),
		viewport.WithHeight(22),
	)
	body.MouseWheelEnabled = true
	body.SoftWrap = false
	// Style padding indents content; viewport shrinks wrap width via
	// GetHorizontalFrameSize so lines stay inside the body column.
	body.Style = viewerStyles.bodySpace.PaddingLeft(1)
	body.KeyMap.Down.SetKeys("down", "j", "ctrl+n")
	body.KeyMap.Up.SetKeys("up", "k", "ctrl+p")
	body.KeyMap.PageDown.SetKeys("pgdown", "space", "ctrl+d")
	body.KeyMap.PageUp.SetKeys("pgup", "b", "ctrl+u")
	body.KeyMap.HalfPageDown.SetEnabled(false)
	body.KeyMap.HalfPageUp.SetEnabled(false)

	v := Viewer{
		document: document,
		styles:   viewerStyles,
		width:    80,
		height:   24,
		viewport: body,
	}
	nav := document.Nav
	v.expanded = defaultExpanded(nav)
	v.focus = focusSidebar
	leaf := ensureLeafActive(nav, nil)
	v.leafPath = leaf
	v.cursorPath = append([]int{}, leaf...)
	v.recomputeLayout()
	return v
}

func (v Viewer) Init() tea.Cmd { return v.viewport.Init() }

func (v Viewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := v.Handle(msg)
	return next, cmd
}

// Handle applies one Bubble Tea message and returns the updated viewer.
func (v Viewer) Handle(msg tea.Msg) (Viewer, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width, v.height = msg.Width, msg.Height
		v.recomputeLayout()
	case tea.KeyPressMsg:
		return v.handleKey(msg)
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			// Divider column sits at x == sideW (between sidebar and body).
			// Allow a 1-cell hit slop on either side for easier grabbing.
			if v.onDivider(msg.X) {
				v.dragging = true
				v.setSideWidth(msg.X)
				return v, nil
			}
			v.click(msg.X, msg.Y)
		}
	case tea.MouseMotionMsg:
		// CellMotion reports motion with a button held — use it for drag-resize.
		// Don't require MouseLeft: some terminals keep the button field sticky.
		if v.dragging {
			v.setSideWidth(msg.X)
		}
	case tea.MouseReleaseMsg:
		// X10 fallback emits release as MouseNone, not MouseLeft — clear on any
		// release while dragging so we never stick in drag mode.
		if v.dragging {
			v.dragging = false
			v.setSideWidth(msg.X)
		}
	case tea.MouseWheelMsg:
		if v.focus == focusSidebar {
			return v, nil
		}
		return v.updateViewport(msg)
	}
	return v, nil
}

// View returns the full-screen Bubble Tea view.
func (v Viewer) View() tea.View {
	view := tea.NewView(v.render())
	view.BackgroundColor = v.styles.bg
	view.AltScreen = true
	// CellMotion: click/release/wheel, plus drag motion while a button is held.
	// Needed so the sidebar divider can be dragged. Tradeoff vs MouseModeNone:
	// the terminal no longer owns highlight-to-copy while the viewer is open
	// (keyboard nav and scroll still work).
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (v *Viewer) refreshRows() {
	if v.expanded == nil {
		v.expanded = defaultExpanded(v.document.Nav)
	}
	v.rows = visibleRows(v.document.Nav, v.expanded)
}

// sidebarItemCapacity is how many nav rows fit under the sidebar header.
func (v Viewer) sidebarItemCapacity() int {
	return max(v.l.viewH-itemsTop, 1)
}

// windowedRows returns the sidebar slice currently on screen and updates offset.
func (v *Viewer) windowedRows() []treeRow {
	cap := v.sidebarItemCapacity()
	cur := selectableRowIndex(v.rows, v.cursorPath)
	win, off := windowRows(v.rows, cur, cap)
	v.windowOffset = off
	return win
}

// activeLeaf is the leaf currently shown in the body (never a group).
func (v *Viewer) activeLeaf() *NavNode {
	return nodeAt(v.document.Nav, v.leafPath)
}

// cursorRow is the index within the full rows list (not the window).
func (v *Viewer) cursorRow() int {
	return selectableRowIndex(v.rows, v.cursorPath)
}

func commonPrefixLen(a, b []int) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
