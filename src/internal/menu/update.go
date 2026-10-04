package menu

import (
	tea "charm.land/bubbletea/v2"
)

func (m model) updateList(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "down", "ctrl+n":
		m.expanded = false
		if n := len(m.matches); n > 0 {
			m.sel = (m.sel + 1) % n
		}
		m.syncList()
		return m, nil

	case "up", "ctrl+p":
		m.expanded = false
		if n := len(m.matches); n > 0 {
			m.sel = (m.sel - 1 + n) % n
		}
		m.syncList()
		return m, nil

	case "tab":
		m.expanded = !m.expanded
		m.syncList()
		return m, nil

	case "enter":
		if len(m.matches) == 0 {
			return m, nil
		}
		task := m.flat[m.matches[m.sel]]
		decision := beginInvocation(task)
		switch decision.kind {
		case collectSubcommandInvocation:
			m.enterSubMode(decision.task)
			return m, nil
		case collectArgumentsInvocation:
			m.enterArgMode(decision.task)
			return m, nil
		case commandInvocation:
			m.chosen = decision.selection()
			return m, tea.Quit
		default:
			return m, nil
		}

	case "right":
		// At the end of the query the right arrow opens the selected row's
		// subcommands, which is how a runnable parent (whose Enter runs it)
		// reaches them; elsewhere it moves the cursor.
		if len(m.matches) > 0 && m.prompt.AtEnd() {
			if task := m.flat[m.matches[m.sel]]; len(task.Children) > 0 {
				m.enterSubMode(task)
				return m, nil
			}
		}

	case "esc":
		switch {
		case m.expanded:
			m.expanded = false
		case m.prompt.Value() != "":
			m.prompt = m.prompt.Reset()
			m.filter()
		case m.sub() != nil:
			m.exitSubMode()
			return m, nil
		default:
			return m, tea.Quit
		}
		m.syncList()
		return m, nil

	case "backspace":
		// An empty query in a subcommand picker backs out one level, mirroring
		// argument-entry's backspace exit; otherwise the key falls through to
		// the prompt's own deletion handling.
		if m.sub() != nil && m.prompt.Value() == "" {
			m.exitSubMode()
			return m, nil
		}
	}

	var cmd tea.Cmd
	before := m.prompt.Value()
	m.prompt, cmd = m.prompt.Update(msg)
	if m.prompt.Value() != before {
		m.expanded = false
		m.filter()
		m.syncList()
	}
	return m, cmd
}

func (m model) updateArgs(msg tea.KeyPressMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if c, ok := m.args.FocusedChip(); ok {
			m.appendChip(c)
			m.args = m.args.ClearChipFocus()
			return m, nil
		}
		return m.submitArgs()

	case "tab":
		m.args = m.args.CycleChip(true)
		return m, nil

	case "shift+tab":
		m.args = m.args.CycleChip(false)
		return m, nil

	case "esc":
		if m.args.ChipFocus() >= 0 {
			m.args = m.args.ClearChipFocus()
			return m, nil
		}
		m.exitArgMode()
		return m, nil

	case "backspace":
		if m.prompt.Value() != "" {
			break
		}
		m.exitArgMode()
		return m, nil
	}

	var cmd tea.Cmd
	before := m.prompt.Value()
	m.prompt, cmd = m.prompt.Update(msg)
	if m.prompt.Value() != before {
		m.args = m.args.DismissErr()
	}
	return m, cmd
}
