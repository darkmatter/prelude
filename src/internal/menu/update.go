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
		case collectArgumentsInvocation:
			m.enterArgMode(decision.task)
			return m, nil
		case commandInvocation:
			m.execCmd = decision.command
			m.hasExecCmd = true
			return m, tea.Quit
		default:
			return m, nil
		}

	case "esc":
		switch {
		case m.expanded:
			m.expanded = false
		case m.prompt.Value() != "":
			m.prompt = m.prompt.Reset()
			m.filter()
		default:
			return m, tea.Quit
		}
		m.syncList()
		return m, nil
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
