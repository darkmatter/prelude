package menu

import "prelude/pkg/ui"

// titleBar renders the panel's title chrome: a half-cell rise from the window
// background into the chrome surface, a centered muted title, and a half-cell
// drop into the open surface below. Presentational — width and surfaces are
// injected at construction/resize; it owns no state beyond its width.
//
// One element per file (React-style): the title bar is its own component, so
// the chrome is no longer a single struct holding title + status + prompt. The
// prompt lives in prompt.go; the status bar in statusbar.go.
type titleBar struct {
	st    styles
	inner int
}

// View renders the three-row title: half-pad up, centered title, half-pad down.
func (t titleBar) View(title string) string {
	st := t.st
	return ui.TitleBar{
		Context:           st.chromeUI,
		Width:             t.inner + 2,
		Outer:             st.bgColor,
		Below:             st.openColor,
		HorizontalPadding: padX,
	}.Render(title)
}

// WithSize returns a copy with the panel's inner width, set on WindowSizeMsg.
func (t titleBar) WithSize(inner int) titleBar { t.inner = inner; return t }
