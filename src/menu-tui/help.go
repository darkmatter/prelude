package main

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/shared"
)

// --- help mode ---------------------------------------------------------------
//
// `menu help` renders a man-style manual generated from the config: a
// contents sidebar, a scrollable body, and an inverted status bar. The
// section list is fixed; COMMANDS and EXAMPLES are filled from the groups.

// helpSectionTitles lists the manual's sections in sidebar/body order.
var helpSectionTitles = []string{
	"name", "synopsis", "description", "options", "commands", "examples", "see also",
}

// helpDoc renders the manual body: styled, unpadded lines wrapped at textW,
// plus the line index where each section header starts.
func (m model) helpDoc(textW int) (lines []string, starts []int) {
	st := m.st
	pal := st.pal
	textW = max(textW, 24)

	insetSp := lipgloss.NewStyle().Background(st.bgColor)
	sp := func(n int) string { return insetSp.Render(strings.Repeat(" ", n)) }

	var out []string
	blank := func() { out = append(out, "") }
	section := func(title string) {
		if len(out) > 0 && out[len(out)-1] != "" {
			blank()
		}
		starts = append(starts, len(out))
		out = append(out, sp(2)+st.inset(pal.Accent).Bold(true).Render(strings.ToUpper(title)))
	}
	para := func(fg shared.Color, indent int, text string) {
		for _, l := range strings.Split(ansi.Wordwrap(text, max(textW-indent, 16), ""), "\n") {
			out = append(out, sp(indent)+st.inset(fg).Render(l))
		}
	}
	// entry renders a bold accent2 term with an indented description, the
	// shape used by OPTIONS and the builtin COMMANDS.
	entry := func(term, desc string) {
		out = append(out, sp(4)+st.inset(pal.Accent2).Bold(true).Render(term))
		para(pal.Muted, 6, desc)
		blank()
	}
	cmdline := func(indent int, cmd string) {
		out = append(out, sp(indent)+st.inset(pal.Accent).Render("$ ")+st.inset(pal.Fg).Render(cmd))
	}

	blank()

	section("name")
	out = append(out, sp(4)+
		st.inset(pal.Accent2).Bold(true).Render("menu")+
		st.inset(pal.Fg).Render(" — interactive command menu for "+m.cfg.Project))

	section("synopsis")
	menuTok := st.inset(pal.Accent2).Bold(true).Render("menu")
	muted := func(s string) string { return st.inset(pal.Muted).Render(s) }
	out = append(out,
		sp(4)+menuTok+muted(" [--config <path>] [")+st.inset(pal.Accent).Render("<task|key>")+muted(" [args…]]"),
		sp(4)+menuTok+st.inset(pal.Fg).Render(" list"),
		sp(4)+menuTok+st.inset(pal.Fg).Render(" help"),
	)

	section("description")
	para(pal.Muted, 4, "menu presents the "+m.cfg.Project+" tasks as a fuzzy-filtered picker: "+
		"type to filter, ↑/↓ to select, ↵ to run, and ⇥ to expand the selected task's details. "+
		"Naming a task (or its key) on the command line skips the picker and runs it directly.")
	blank()
	para(pal.Muted, 4, "Tasks that declare arguments open argument-entry mode with suggested "+
		"value chips, boolean flags, required-argument validation, and a live command preview. "+
		"Extra command-line arguments bypass argument entry and are appended verbatim.")
	blank()
	if m.cfg.Execute {
		para(pal.Muted, 4, "The selected command replaces the menu process (bash -c) and the menu exits with its status.")
	} else {
		para(pal.Muted, 4, "This menu is configured to print the assembled command instead of executing it.")
	}

	section("options")
	entry("--config <path>", "Path to the menu config JSON. Defaults to $PRELUDE_MENU_CONFIG; the Nix wrapper bakes this in.")
	entry("PRELUDE_MENU_DEBUG=<path>", "Write TUI diagnostics to the given file.")

	section("commands")
	entry("list", "Print the grouped task table and exit (non-interactive).")
	entry("help", "Show this manual.")
	for _, g := range m.cfg.Groups {
		if g.Title != "" {
			out = append(out, sp(4)+st.inset(pal.Muted).Render(letterSpace(g.Title)))
			blank()
		}
		for _, t := range g.Tasks {
			nameLine := sp(4) + st.inset(pal.Accent2).Bold(true).Render(t.Name)
			if t.Key != "" {
				nameLine += st.inset(pal.Dim).Render("  (" + t.Key + ")")
			}
			out = append(out, nameLine)
			if t.Description != "" {
				para(pal.Muted, 6, t.Description)
			}
			if t.Details != "" {
				para(pal.Muted, 6, t.Details)
			}
			if t.Usage != "" {
				cmdline(6, t.Usage)
			}
			blank()
		}
	}

	section("examples")
	example := func(cmd, desc string) {
		cmdline(4, cmd)
		if desc != "" {
			para(pal.Dim, 6, desc)
		}
		blank()
	}
	example("menu", "open the interactive picker")
	example("menu list", "print the task table without a TTY")
	for _, g := range m.cfg.Groups {
		for _, t := range g.Tasks {
			for _, ex := range t.Examples {
				example(ex, "")
			}
		}
	}

	section("see also")
	out = append(out, sp(4)+st.inset(pal.Fg).Render("menu list")+
		st.inset(pal.Muted).Render(" — the same catalogue as a plain table"))
	out = append(out, sp(4)+st.inset(pal.Fg).Render("⇥ in the picker")+
		st.inset(pal.Muted).Render(" — per-task details, usage, and examples"))
	blank()

	return out, starts
}

// --- navigation ----------------------------------------------------------------

func (m model) updateHelp(msg tea.KeyPressMsg) (model, tea.Cmd) {
	l := m.helpLayout()
	switch msg.String() {
	case "q", "esc":
		return m, tea.Quit
	case "j", "down", "ctrl+n":
		m.helpScrollBy(1)
	case "k", "up", "ctrl+p":
		m.helpScrollBy(-1)
	case "pgdown", "space", "ctrl+d":
		m.helpScrollBy(l.viewH)
	case "pgup", "b", "ctrl+u":
		m.helpScrollBy(-l.viewH)
	case "home", "g":
		m.helpScroll, m.helpActive = 0, 0
	case "end", "G", "shift+g":
		m.helpScrollBy(1 << 20)
	default:
		if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
			m.helpJumpSection(int(s[0] - '1'))
		}
	}
	return m, nil
}

// helpScrollBy moves the viewport and re-derives the active section from the
// topmost visible line.
func (m *model) helpScrollBy(delta int) {
	l := m.helpLayout()
	lines, starts := m.helpDoc(l.textW)
	maxScroll := max(0, len(lines)-l.viewH)
	m.helpScroll = min(max(m.helpScroll+delta, 0), maxScroll)
	m.helpActive = helpActiveAt(starts, m.helpScroll)
}

// helpJumpSection scrolls a section's header to the top of the viewport and
// marks it active even when the tail sections cannot reach the top line.
func (m *model) helpJumpSection(i int) {
	if i < 0 || i >= len(helpSectionTitles) {
		return
	}
	l := m.helpLayout()
	lines, starts := m.helpDoc(l.textW)
	maxScroll := max(0, len(lines)-l.viewH)
	m.helpScroll = min(starts[i], maxScroll)
	m.helpActive = i
}

// helpClick maps a terminal click to a sidebar item.
func (m *model) helpClick(x, y int) {
	l := m.helpLayout()
	if x >= l.sideW {
		return
	}
	if i := y - helpSidebarItemsTop; i >= 0 && i < len(helpSectionTitles) {
		m.helpJumpSection(i)
	}
}

// helpActiveAt returns the section covering the given top line.
func helpActiveAt(starts []int, off int) int {
	active := 0
	for i, s := range starts {
		if s <= off {
			active = i
		}
	}
	return active
}
