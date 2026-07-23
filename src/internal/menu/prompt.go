package menu

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Prompt is the filter/argument input row: an amber caret and a real bar-cursor
// textinput, sitting on the open surface outside the framed body.
//
// It is a single-purpose bubbletea sub-model: it owns only the input state and
// its rendering. The root menu model owns the Prompt, routes keypresses through
// it, and reads Value() to re-filter or to build the invocation. Mode-specific
// context labels and placeholders live in the root and are passed to View/Cursor
// at render time, so Prompt never switches roles — it just renders whatever
// context and placeholder the root asks for.
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
}

// newPrompt builds the input with the open-surface styles and a blinking
// terminal bar cursor, focused and ready. The initial width is computed from
// a default context so the textinput has a valid viewport before the first
// View call.
func newPrompt(st styles, inner int) Prompt {
	context := "~/prelude"
	in := textinput.New()
	in.Prompt = ""
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
// width. The context label and placeholder are owned by the root; Prompt is
// only responsible for turning them into a row.
func (p Prompt) View(context, placeholder string) string {
	p.input.Placeholder = placeholder
	p.input.SetWidth(max(p.inner+2-padX-lipgloss.Width(context)-4, 1))
	row := p.openSp.PaddingLeft(padX).Render("") +
		p.openMuted.Render(context) +
		p.openSp.Render(" ") +
		p.openCaret.Bold(true).Render("❯") +
		p.openSp.Render(" ") +
		p.input.View()
	return p.openSp.Width(p.inner + 2).MaxWidth(p.inner + 2).Render(row)
}

// Cursor returns the input's terminal cursor relative to the prompt row. The
// root model adds the prompt row and centered-panel offsets before rendering.
func (p Prompt) Cursor(context string) *tea.Cursor {
	p.input.SetWidth(max(p.inner+2-padX-lipgloss.Width(context)-4, 1))
	cursor := p.input.Cursor()
	if cursor == nil {
		return nil
	}
	cursor.Position.X += padX + lipgloss.Width(context) + 3
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

// WithSize updates the panel width and the input's real-cursor viewport for
// the given context width. The root owns the context label and passes it here
// whenever the panel is resized or the context changes.
func (p Prompt) WithSize(inner int, context string) Prompt {
	p.inner = inner
	p.input.SetWidth(max(p.inner+2-padX-lipgloss.Width(context)-4, 1))
	return p
}
