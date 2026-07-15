package main

import (
	"fmt"
	"io"
	"os"
	"prelude/shared"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// printList renders the grouped task table non-interactively. The profile
// writer preserves terminal color while stripping ANSI when output is piped.
func printList(cfg *Config, st styles) {
	printListTo(os.Stdout, os.Environ(), cfg, st)
}

func printListTo(output io.Writer, environ []string, cfg *Config, st styles) {
	width := 80
	if file, ok := output.(interface{ Fd() uintptr }); ok {
		if w, _, err := term.GetSize(int(file.Fd())); err == nil && w > 0 {
			width = w
		}
	}
	if cfg.MaxWidth > 0 && width > cfg.MaxWidth {
		width = cfg.MaxWidth
	}

	w := shared.ColorWriter(output, environ, cfg.ColorProfile)

	first := true
	for _, g := range cfg.Groups {
		if len(g.Tasks) == 0 {
			continue
		}
		if !first {
			fmt.Fprintln(w)
		}
		first = false
		if g.Title != "" {
			fmt.Fprintln(w, st.muted.Render(letterSpace(g.Title)))
		}
		for _, t := range g.Tasks {
			fmt.Fprintln(w, listRow(st, t, width))
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, st.dim.Render("run menu to pick a task interactively"))
}

// listRow paints one non-interactive task line in the same language as the
// picker: optional key rail, bold name, muted description, optional right hint.
func listRow(st styles, t Task, width int) string {
	keyLabel := ""
	if t.Key != "" {
		keyLabel = t.Key
	}
	marker := ""
	if len(t.Args) > 0 {
		marker = "◆ args"
	}

	leftPad := st.fg.Render("  ")
	shortcut := strings.Repeat(" ", 3)
	if keyLabel != "" {
		// Plain (no bg) stand-in for the framed keycap used in the TUI.
		shortcut = st.accent2.Bold(true).Render(" " + keyLabel + " ")
		if lipgloss.Width(shortcut) < 3 {
			shortcut += strings.Repeat(" ", 3-lipgloss.Width(shortcut))
		}
	}
	name := st.fg.Bold(true).Render(t.Name)
	desc := t.Description

	used := lipgloss.Width(leftPad) + lipgloss.Width(shortcut) + 1 + lipgloss.Width(name) + 1 +
		lipgloss.Width(marker) + 1
	descBudget := max(width-used, 4)
	descRendered := st.muted.Render(ansi.Truncate(desc, descBudget, "…"))

	line := leftPad + shortcut + " " + name + " " + descRendered
	pad := width - lipgloss.Width(line) - lipgloss.Width(marker)
	if pad < 1 {
		pad = 1
	}
	markerStyle := st.dim
	if len(t.Args) > 0 {
		markerStyle = st.accent2
	}
	return line + strings.Repeat(" ", pad) + markerStyle.Render(marker)
}
