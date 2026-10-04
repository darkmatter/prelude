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
	m.promptCtx = m.listPromptCtx()
	m.promptPlaceholder = m.cfg.Placeholder
	m.prompt = m.prompt.Reset().WithSize(m.layout.inner, m.promptCtx)
	m.filter()
	m.syncList()
}

// sub returns the parent whose subcommands the list shows, nil at the root.
func (m model) sub() *Task {
	if len(m.pickers) == 0 {
		return nil
	}
	return &m.pickers[len(m.pickers)-1].parent
}

// listPromptCtx is the prompt context of the list on screen: the open
// picker's parent, else the project.
func (m model) listPromptCtx() string {
	if sub := m.sub(); sub != nil {
		return sub.displayName()
	}
	return "~/" + m.cfg.Project
}

// enterSubMode swaps the list to one parent task's subcommands, pushing a
// frame that snapshots the current list so leaving returns to the same
// filtered position. Pickers nest: a child with its own children opens
// another frame on top.
func (m *model) enterSubMode(task Task) {
	matches := make([]int, len(m.matches))
	copy(matches, m.matches)
	// The full slice expression copies on push, so an earlier model value
	// never sees a frame pushed after it.
	m.pickers = append(m.pickers[:len(m.pickers):len(m.pickers)], listFrame{
		parent:            task,
		flat:              m.flat,
		matches:           matches,
		sel:               m.sel,
		expanded:          m.expanded,
		promptValue:       m.prompt.Value(),
		promptCtx:         m.promptCtx,
		promptPlaceholder: m.promptPlaceholder,
	})
	// A task can arrive straight from the Config (`x <parent>`), so its
	// children may not carry filter haystacks yet.
	m.flat = searchable(task.Children)
	m.matches = nil
	m.sel = 0
	m.expanded = false
	m.promptCtx = task.displayName()
	m.promptPlaceholder = m.cfg.Placeholder
	m.prompt = m.prompt.Reset().WithSize(m.layout.inner, m.promptCtx)
	m.filter()
	m.syncList()
}

// exitSubMode closes the innermost picker and restores the list it replaced.
func (m *model) exitSubMode() {
	if len(m.pickers) == 0 {
		return
	}
	saved := m.pickers[len(m.pickers)-1]
	m.pickers = m.pickers[:len(m.pickers)-1]
	m.flat = saved.flat
	m.matches = saved.matches
	m.sel = saved.sel
	m.expanded = saved.expanded
	m.promptCtx = saved.promptCtx
	m.promptPlaceholder = saved.promptPlaceholder
	m.prompt = m.prompt.Reset().WithValue(saved.promptValue).WithCursorEnd().WithSize(m.layout.inner, m.promptCtx)
	m.syncList()
}

// focus selects the listed task named name, when the filter shows it.
func (m *model) focus(name string) {
	for position, index := range m.matches {
		if m.flat[index].Name == name {
			m.sel = position
			m.syncList()
			return
		}
	}
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
	chosen, err := m.args.Submit(m.prompt.Value())
	if err != nil {
		m.args = m.args.SetErr(err.Error())
		return m, nil
	}
	m.chosen = chosen
	return m, tea.Quit
}

func argPlaceholder(t Task) string {
	tokens := make([]string, len(t.Args))
	for i, a := range t.Args {
		tokens[i] = a.Token
	}
	return strings.Join(tokens, " ")
}
