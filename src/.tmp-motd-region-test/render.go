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

// renderBody composes three sibling surfaces at one shared card width:
// header → divider on window/default → content container.
func (r renderer) renderBody() string {
	var sections []string

	for range max(r.cfg.Padding.Top, 0) {
		sections = append(sections, r.headerBlankLine())
	}
	if header := r.renderHeader(); header != "" {
		sections = append(sections, header)
	}

	// The divider belongs to neither sibling. The blank row below it is the
	// first row owned by the content container.
	sections = append(sections, r.renderHeaderDivider(), r.blankLine())

	h := r.cfg.Header
	if h.Tagline != "" || h.Subtitle != "" {
		sections = append(sections, join(r.renderActivation(h.Tagline, h.Subtitle)...))
	}

	if middle := r.renderMiddle(); middle != "" {
		sections = append(sections, middle)
	}

	if shortcuts := r.renderShortcuts(); shortcuts != "" {
		sections = append(sections, r.blankLine(), shortcuts)
	}

	// Bottom padding is under the whole card — below the shortcut hints when
	// present, not between middle content and the hints.
	for range max(r.cfg.Padding.Bottom, 0) {
		sections = append(sections, r.blankLine())
	}

	return r.joinCardVertical(sections...)
}

// renderMiddle builds description + env + getting-started, then applies side
// padding. Vertical padding is applied around the whole card in renderBody.
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
