package main

import (
	"strings"
)

// render produces the full MOTD for a terminal of the given width.
func render(cfg Config, terminalWidth int, runtime Runtime) string {
	r := newRenderer(cfg, terminalWidth, runtime)
	output := r.renderWindow(r.renderBody())
	if !cfg.ClearScreen {
		return output
	}
	return "\x1b[2J\x1b[H" + output
}

// renderBody composes the card from playground-aligned sections:
// header bar → description → env → Getting Started → shortcuts.
func (r renderer) renderBody() string {
	var sections []string

	if header := r.renderHeader(); header != "" {
		sections = append(sections, header, r.blankLine())
	}

	if middle := r.renderMiddle(); middle != "" {
		sections = append(sections, middle)
	}

	if shortcuts := r.renderShortcuts(); shortcuts != "" {
		sections = append(sections, r.blankLine(), shortcuts)
	}

	return r.joinCardVertical(sections...)
}

// renderMiddle builds description + env + getting-started, then applies padding.
func (r renderer) renderMiddle() string {
	var content block

	if desc := r.renderDescription(); len(desc) > 0 {
		content.writeSection(desc)
	}

	if env := r.renderEnv(); len(env) > 0 {
		content.writeSection(env)
	}

	if started := r.renderGettingStarted(); len(started) > 0 {
		content.writeLines(started)
	}

	return r.padContent(strings.TrimSuffix(content.String(), "\n"))
}
