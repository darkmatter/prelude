package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// renderEnv builds tool versions + optional git status as one flowing row.
func (r renderer) renderEnv() []string {
	var row strings.Builder
	for _, item := range r.cfg.Env {
		if rendered, ok := r.renderEnvItem(item); ok {
			row.WriteString(rendered)
		}
	}
	row.WriteString(r.renderGitStatus())

	if strings.TrimSpace(ansi.Strip(row.String())) == "" {
		return nil
	}
	return r.wrapAndFill(row.String(), r.contentWidth)
}

func (r renderer) renderEnvItem(item EnvItem) (string, bool) {
	value := item.Value
	if item.Probe != "" {
		probed, err := r.runtime.Probe(item.Probe)
		if err != nil || probed == "" {
			return "", false
		}
		value = probed
	}
	return inline(r.st.dim).Render(item.Label+" ") +
		inline(r.st.fgBold).Render(value+"   "), true
}

func (r renderer) renderGitStatus() string {
	if !r.cfg.Git {
		return ""
	}
	git, ok := r.runtime.Git()
	if !ok {
		return ""
	}
	return inline(r.st.dim).Render("git ") +
		inline(r.st.amber).Bold(true).Render(git.Branch) +
		inline(r.st.muted).Render(fmt.Sprintf(" ↑%d ●%d", git.Ahead, git.Dirty))
}

// wrapAndFill wraps an already-styled string on display width, then pads each line.
func (r renderer) wrapAndFill(value string, width int) []string {
	s := r.st.blockFill.Width(width).MaxWidth(width)
	var bl block
	for _, line := range strings.Split(ansi.Wrap(value, width, ""), "\n") {
		bl.write(s.Render(line))
	}
	return splitLines(bl.String())
}
