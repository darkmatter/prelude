// Package manual renders and navigates Prelude manuals through one shared viewer.
package manual

import (
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"prelude/pkg/shared"
)

// Viewer owns manual state, navigation, layout, chrome, and presentation.
type Viewer struct {
	document Document
	styles   styles

	width    int
	height   int
	active   int
	viewport viewport.Model

	// Cached on WindowSizeMsg so section jumps do not need to re-render the
	// document on every keypress.
	l      layout
	starts []int
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
	body.Style = viewerStyles.bodySpace
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
			v.click(msg.X, msg.Y)
		}
	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		v.viewport, cmd = v.viewport.Update(msg)
		return v, cmd
	}
	return v, nil
}

// View returns the full-screen Bubble Tea view.
func (v Viewer) View() tea.View {
	view := tea.NewView(v.render())
	view.BackgroundColor = v.styles.bg
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}
