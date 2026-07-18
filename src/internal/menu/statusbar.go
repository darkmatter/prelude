package menu

import (
	"strings"

	"prelude/pkg/ui"
)

// statusBar renders the panel's status chrome: a half-cell rise from the open
// surface, the keymap hints (left) and live status (right) on the chrome
// surface, and a half-cell drop back to the window background. Presentational.
//
// One element per file (React-style): the status bar is its own component,
// split out of the old chrome struct alongside the title bar (titlebar.go) and
// the prompt (prompt.go).
type statusBar struct {
	st    styles
	inner int
}

// View renders the three-row status footer: half-pad up, keymap + status, half-pad down.
func (s statusBar) View(hints [][2]string, status string) string {
	st := s.st

	statusStyle := st.chromeUI.Success().Bold(true)
	if strings.Contains(status, "args") {
		statusStyle = st.chromeUI.Info().Bold(true)
	}
	keyStyle := st.kbdChip
	footer := ui.KeyHintsFooter{
		Context:           st.chromeUI,
		Width:             s.inner + 2,
		Outer:             st.bgColor,
		Open:              st.openColor,
		Key:               &keyStyle,
		Status:            &statusStyle,
		HorizontalPadding: padX,
	}

	genericHints := make([]ui.KeyHint, len(hints))
	for i, hint := range hints {
		genericHints[i] = ui.KeyHint{Key: hint[0], Text: hint[1]}
	}
	return footer.Render(genericHints, status)
}

// WithSize returns a copy with the panel's inner width, set on WindowSizeMsg.
func (s statusBar) WithSize(inner int) statusBar { s.inner = inner; return s }
