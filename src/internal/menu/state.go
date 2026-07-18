package menu

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *model) filter() {
	q := strings.ToLower(strings.TrimSpace(m.prompt.Value()))
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
	m.args = m.args.EnterArg(t)
	m.prompt = m.prompt.Reset().WithPlaceholder(argPlaceholder(t)).WithContext(t.displayName())
}

func (m *model) exitArgMode() {
	m.mode = modeList
	m.args = m.args.ExitArg()
	m.prompt = m.prompt.Reset().WithPlaceholder(m.cfg.Placeholder).WithContext("~/" + m.cfg.Project)
	m.filter()
	m.syncList()
}

func (m *model) appendChip(c chip) {
	token := invocationToken(c.arg, c.value)
	v := m.prompt.Value()
	if v != "" && !strings.HasSuffix(v, " ") {
		v += " "
	}
	m.prompt = m.prompt.WithValue(v + token).WithCursorEnd()
	m.args = m.args.DismissErr()
}

func (m model) submitArgs() (model, tea.Cmd) {
	decision, err := completeInvocation(*m.args.Task(), m.prompt.Value())
	if err != nil {
		m.args = m.args.SetErr(err.Error())
		return m, nil
	}
	m.execCmd = decision.command
	m.hasExecCmd = true
	return m, tea.Quit
}

func argPlaceholder(t Task) string {
	tokens := make([]string, len(t.Args))
	for i, a := range t.Args {
		tokens[i] = a.Token
	}
	return strings.Join(tokens, " ")
}
