package main

import (
	"fmt"
	"log"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type mode int

const (
	modeList mode = iota
	modeArgs
)

// chip is one selectable suggested value in argument-entry mode.
type chip struct {
	arg   Arg
	label string
	value string
}

type model struct {
	cfg *Config
	st  styles

	flat    []Task
	input   textinput.Model // filter query (list) / arg string (args)
	matches []int           // indices into flat, group order preserved
	sel     int             // index into matches
	offset  int             // list scroll offset (in rendered lines)

	expanded bool
	mode     mode

	argTask   *Task
	argErr    string
	chips     []chip
	chipFocus int // -1 = input focused

	width, height int
	execCmd       string // set on selection; consumed by main after quit
}

func newModel(cfg *Config, st styles, argTask *Task) model {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = cfg.Placeholder
	inputStyles := textinput.DefaultDarkStyles()
	inputStyles.Focused.Placeholder = st.sDim
	inputStyles.Focused.Text = st.sFg
	inputStyles.Blurred.Placeholder = st.sDim
	inputStyles.Blurred.Text = st.sFg
	inputStyles.Cursor.Color = st.accentC
	inputStyles.Cursor.Blink = true
	in.SetStyles(inputStyles)
	in.SetVirtualCursor(true)
	in.Focus()

	m := model{
		cfg:       cfg,
		st:        st,
		flat:      cfg.flatten(),
		input:     in,
		chipFocus: -1,
		width:     80,
		height:    24,
	}
	m.filter()
	if argTask != nil {
		m.enterArgMode(*argTask)
	}
	return m
}

func (m model) Init() tea.Cmd { return textinput.Blink }

// ---------------------------------------------------------------------------
// update
// ---------------------------------------------------------------------------

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case tea.KeyPressMsg:
		if debugLog {
			log.Printf("key=%q mode=%d sel=%d matches=%d", msg.String(), m.mode, m.sel, len(m.matches))
		}
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.mode == modeArgs {
			return m.updateArgs(msg)
		}
		return m.updateList(msg)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "down", "ctrl+n":
		m.expanded = false
		if n := len(m.matches); n > 0 {
			m.sel = (m.sel + 1) % n
		}
		return m, nil

	case "up", "ctrl+p":
		m.expanded = false
		if n := len(m.matches); n > 0 {
			m.sel = (m.sel - 1 + n) % n
		}
		return m, nil

	case "tab":
		m.expanded = !m.expanded
		return m, nil

	case "enter":
		if len(m.matches) == 0 {
			return m, nil
		}
		t := m.flat[m.matches[m.sel]]
		if len(t.Args) > 0 {
			m.enterArgMode(t)
			return m, nil
		}
		m.execCmd = t.Run
		return m, tea.Quit

	case "esc":
		switch {
		case m.expanded:
			m.expanded = false
		case m.input.Value() != "":
			m.input.SetValue("")
			m.filter()
		default:
			return m, tea.Quit
		}
		return m, nil
	}

	var cmd tea.Cmd
	before := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.expanded = false
		m.filter()
	}
	return m, cmd
}

func (m model) updateArgs(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if m.chipFocus >= 0 {
			m.appendChip(m.chips[m.chipFocus])
			m.chipFocus = -1
			return m, nil
		}
		return m.submitArgs()

	case "tab":
		if len(m.chips) > 0 {
			m.chipFocus = (m.chipFocus + 1) % (len(m.chips) + 1)
			if m.chipFocus == len(m.chips) {
				m.chipFocus = -1
			}
		}
		return m, nil

	case "shift+tab":
		if len(m.chips) > 0 {
			if m.chipFocus < 0 {
				m.chipFocus = len(m.chips) - 1
			} else {
				m.chipFocus--
			}
		}
		return m, nil

	case "esc":
		if m.chipFocus >= 0 {
			m.chipFocus = -1
			return m, nil
		}
		m.exitArgMode()
		return m, nil

	case "backspace":
		if m.input.Value() != "" {
			break
		}
		m.exitArgMode()
		return m, nil
	}

	var cmd tea.Cmd
	before := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.argErr = ""
	}
	return m, cmd
}

// ---------------------------------------------------------------------------
// state transitions
// ---------------------------------------------------------------------------

func (m *model) filter() {
	q := strings.ToLower(strings.TrimSpace(m.input.Value()))
	m.matches = m.matches[:0]
	for i, t := range m.flat {
		if q == "" || strings.Contains(t.haystack, q) {
			m.matches = append(m.matches, i)
		}
	}
	if m.sel >= len(m.matches) {
		m.sel = max(0, len(m.matches)-1)
	}
}

func (m *model) enterArgMode(t Task) {
	m.mode = modeArgs
	m.argTask = &t
	m.argErr = ""
	m.chipFocus = -1
	m.chips = nil
	for _, a := range t.Args {
		for _, opt := range a.Options {
			label := opt
			if a.Boolean {
				label = a.Token
			}
			m.chips = append(m.chips, chip{arg: a, label: label, value: opt})
		}
		if a.Boolean && len(a.Options) == 0 {
			m.chips = append(m.chips, chip{arg: a, label: a.Token, value: a.Token})
		}
	}
	m.input.SetValue("")
	m.input.Placeholder = argPlaceholder(t)
}

func (m *model) exitArgMode() {
	m.mode = modeList
	m.argTask = nil
	m.argErr = ""
	m.chipFocus = -1
	m.input.SetValue("")
	m.input.Placeholder = m.cfg.Placeholder
	m.filter()
}

func (m *model) appendChip(c chip) {
	token := tokenFor(c.arg, c.value)
	v := m.input.Value()
	if v != "" && !strings.HasSuffix(v, " ") {
		v += " "
	}
	m.input.SetValue(v + token)
	m.input.CursorEnd()
	m.argErr = ""
}

func (m model) submitArgs() (model, tea.Cmd) {
	t := m.argTask
	val := strings.TrimSpace(m.input.Value())
	if val == "" {
		for _, a := range t.Args {
			if a.Required {
				m.argErr = fmt.Sprintf("%s: missing required argument %s", t.Name, a.Token)
				return m, nil
			}
		}
	}
	cmd := t.Run
	if val != "" {
		cmd += " " + val
	}
	m.execCmd = cmd
	return m, tea.Quit
}

func argPlaceholder(t Task) string {
	tokens := make([]string, len(t.Args))
	for i, a := range t.Args {
		tokens[i] = a.Token
	}
	return strings.Join(tokens, " ")
}
