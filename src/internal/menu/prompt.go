package menu

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Prompt is the filter/argument input row: a project or task context, an amber
// caret, and a real bar-cursor textinput, sitting on the open surface outside
// the framed body.
//
// It is a self-contained bubbletea sub-model: it owns its own input state and
// forwards messages to the embedded textinput. The root menu model owns a
// Prompt, routes keypresses through it, and reads Value() to re-filter or to
// build the invocation. The rendered prompt row — context, caret, input —
// lives here too, so the element owns both its state and its view (one file
// per element, React-style).
//
// Value semantics mirror textinput.Model: Update returns a new Prompt, the
// other mutators return a copy with the change applied, so the caller writes
// `m.prompt = m.prompt.Reset()`.
type Prompt struct {
	input     textinput.Model
	openSp    lipgloss.Style // open surface fill (outside the frame)
	openMuted lipgloss.Style // context label
	openCaret lipgloss.Style // amber ❯
	inner     int            // framed body width; the row spans inner+2
	context   string         // "~/project" in list mode, task name in arg mode
}

// newPrompt builds the filter/arg input with the open-surface styles and a
// blinking terminal bar cursor, focused and ready for list mode. The initial
// placeholder is the list-mode filter hint (cfg.Placeholder); arg mode swaps
// it for the token list via WithPlaceholder.
func newPrompt(st styles, project, placeholder string, inner int) Prompt {
	context := "~/" + project
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = placeholder
	in.SetStyles(textinputStyles(st))
	in.SetVirtualCursor(false)
	// Reserve one cell after the text viewport for the insertion cursor.
	in.SetWidth(max(inner+2-padX-lipgloss.Width(context)-4, 1))
	in.Focus()
	return Prompt{
		input:     in,
		openSp:    st.openSp,
		openMuted: st.openMuted,
		openCaret: st.openAccent2,
		inner:     inner,
		context:   context,
	}
}

// textinputStyles wires the embedded input to the open-surface palette. A real
// cursor is required because Bubbles renders every virtual cursor as a block.
func textinputStyles(st styles) textinput.Styles {
	s := textinput.DefaultDarkStyles()
	s.Focused.Placeholder = st.openDim
	s.Focused.Text = st.openFg
	s.Blurred.Placeholder = st.openDim
	s.Blurred.Text = st.openFg
	s.Cursor.Color = st.accentC
	s.Cursor.Shape = tea.CursorBar
	s.Cursor.Blink = true
	return s
}

// Init is idle because the terminal owns blinking for a real cursor.
func (p Prompt) Init() tea.Cmd { return nil }

// Update forwards a message to the embedded textinput and returns the
// updated Prompt. The root model decides whether a value change implies a
// re-filter; it reads Value() before and after.
func (p Prompt) Update(msg tea.Msg) (Prompt, tea.Cmd) {
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

// View renders the prompt row: left pad, context, amber caret, then the
// input's own view, all on the open surface and constrained to the panel
// width. This is the row the chrome used to draw as promptRow.
func (p Prompt) View() string {
	row := p.openSp.PaddingLeft(padX).Render("") +
		p.openMuted.Render(p.context) +
		p.openSp.Render(" ") +
		p.openCaret.Bold(true).Render("❯") +
		p.openSp.Render(" ") +
		p.input.View()
	return p.openSp.Width(p.inner + 2).MaxWidth(p.inner + 2).Render(row)
}

// Cursor returns the input's terminal cursor relative to the prompt row. The
// root model adds the prompt row and centered-panel offsets before rendering.
func (p Prompt) Cursor() *tea.Cursor {
	cursor := p.input.Cursor()
	if cursor == nil {
		return nil
	}
	cursor.Position.X += padX + lipgloss.Width(p.context) + 3
	return cursor
}

// Value is the current input text.
func (p Prompt) Value() string { return p.input.Value() }

// Reset clears the input value, returning a copy.
func (p Prompt) Reset() Prompt { p.input.Reset(); return p }

// WithPlaceholder sets the input's placeholder (the list-mode filter hint or
// the arg-mode token list), returning a copy.
func (p Prompt) WithPlaceholder(s string) Prompt { p.input.Placeholder = s; return p }

// WithValue replaces the input text, returning a copy.
func (p Prompt) WithValue(s string) Prompt { p.input.SetValue(s); return p }

// WithCursorEnd moves the cursor to the end, returning a copy.
func (p Prompt) WithCursorEnd() Prompt { p.input.CursorEnd(); return p }

// WithContext sets the context label shown left of the caret (task name in
// arg mode, "~/project" in list mode), returning a copy. Recomputing the input
// width keeps long values horizontally scrolled inside the prompt row.
func (p Prompt) WithContext(s string) Prompt {
	p.context = s
	p.input.SetWidth(max(p.inner+2-padX-lipgloss.Width(p.context)-4, 1))
	return p
}

// WithSize updates the panel width and the input's real-cursor viewport.
func (p Prompt) WithSize(inner int) Prompt {
	p.inner = inner
	p.input.SetWidth(max(p.inner+2-padX-lipgloss.Width(p.context)-4, 1))
	return p
}
