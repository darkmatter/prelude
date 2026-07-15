package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// renderEnv builds tool versions as one flowing row of chips.
func (r renderer) renderEnv() []string {
	var row strings.Builder
	for _, item := range r.cfg.Env {
		if rendered, ok := r.renderEnvItem(item); ok {
			row.WriteString(rendered)
		}
	}

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

// wrapAndFill wraps an already-styled string on display width, then pads each line.
func (r renderer) wrapAndFill(value string, width int) []string {
	s := r.st.blockFill.Width(width).MaxWidth(width)
	var bl block
	for _, line := range strings.Split(ansi.Wrap(value, width, ""), "\n") {
		bl.write(s.Render(line))
	}
	return splitLines(bl.String())
}
