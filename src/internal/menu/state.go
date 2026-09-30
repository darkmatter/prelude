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
	m.promptCtx = t.displayName()
	m.promptPlaceholder = argPlaceholder(t)
	m.prompt = m.prompt.Reset().WithSize(m.layout.inner, m.promptCtx)
}

func (m *model) exitArgMode() {
	m.mode = modeList
	m.args = m.args.ExitArg()
	m.promptCtx = "~/" + m.cfg.Project
	m.promptPlaceholder = m.cfg.Placeholder
	m.prompt = m.prompt.Reset().WithSize(m.layout.inner, m.promptCtx)
	m.filter()
	m.syncList()
}

// enterSubMode swaps the list to one parent task's subcommands. The root list
// state is snapshotted so esc returns to the same filtered position.
func (m *model) enterSubMode(task Task) {
	matches := make([]int, len(m.matches))
	copy(matches, m.matches)
	m.saved = &listFrame{
		flat:              m.flat,
		matches:           matches,
		sel:               m.sel,
		expanded:          m.expanded,
		promptValue:       m.prompt.Value(),
		promptCtx:         m.promptCtx,
		promptPlaceholder: m.promptPlaceholder,
	}
	m.sub = &task
	m.flat = task.Children
	m.matches = nil
	m.sel = 0
	m.expanded = false
	m.promptCtx = task.displayName()
	m.promptPlaceholder = m.cfg.Placeholder
	m.prompt = m.prompt.Reset().WithSize(m.layout.inner, m.promptCtx)
	m.filter()
	m.syncList()
}

// exitSubMode restores the snapshotted root list and drops the submenu state.
func (m *model) exitSubMode() {
	if m.saved == nil {
		return
	}
	saved := *m.saved
	m.saved = nil
	m.sub = nil
	m.flat = saved.flat
	m.matches = saved.matches
	m.sel = saved.sel
	m.expanded = saved.expanded
	m.promptCtx = saved.promptCtx
	m.promptPlaceholder = saved.promptPlaceholder
	m.prompt = m.prompt.Reset().WithValue(saved.promptValue).WithCursorEnd().WithSize(m.layout.inner, m.promptCtx)
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
	cmd, err := m.args.Submit(m.prompt.Value())
	if err != nil {
		m.args = m.args.SetErr(err.Error())
		return m, nil
	}
	m.execCmd = cmd
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
