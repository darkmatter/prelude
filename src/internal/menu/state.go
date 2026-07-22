package menu

import (
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
)

func (m *model) filter() {
	// Trim only — sahilm/fuzzy (via bubbles list.DefaultFilter) is already
	// case-insensitive. Empty query means the full catalogue.
	q := strings.TrimSpace(m.prompt.Value())
	m.matches = m.matches[:0]
	if q == "" {
		for i := range m.flat {
			m.matches = append(m.matches, i)
		}
	} else {
		targets := make([]string, len(m.flat))
		for i, t := range m.flat {
			targets[i] = t.haystack
		}
		// UnsortedFilter uses the same sahilm/fuzzy engine as DefaultFilter but
		// keeps catalogue/group order so grouped headers stay stable while
		// typing. Ranked reordering would scatter groups mid-list.
		for _, rank := range list.UnsortedFilter(q, targets) {
			m.matches = append(m.matches, rank.Index)
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
