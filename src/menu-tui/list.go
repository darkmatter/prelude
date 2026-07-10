package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
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

	flat := cfg.flatten()
	nameW, hintW := 4, 2
	for _, t := range flat {
		nameW = max(nameW, lipgloss.Width(t.Name))
		hintW = max(hintW, lipgloss.Width(listHint(t)))
	}
	nameW += 4
	hintW = min(hintW+2, 28)
	descW := max(width-nameW-hintW-2, 10)

	nameStyle := st.fg.Bold(true).Width(nameW)
	descStyle := st.muted.Width(descW)
	hintStyle := st.dim.Width(hintW).Align(lipgloss.Right)

	w := colorWriter(output, environ, cfg.ColorProfile)

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
			fmt.Fprintln(w, st.accent2.Bold(true).Render("["+g.Title+"]"))
		}
		for _, t := range g.Tasks {
			row := lipgloss.JoinHorizontal(
				lipgloss.Top,
				nameStyle.Render("  "+t.Name),
				descStyle.Render(t.Description),
				"  ",
				hintStyle.Render(listHint(t)),
			)
			fmt.Fprintln(w, row)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, st.dim.Render("run menu to pick a task interactively"))
}

func listHint(t Task) string {
	if t.Key != "" {
		return "⌨ " + t.Key
	}
	return t.Run
}

func padRight(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}
