package main

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Title style variants for the header wordmark (playground HeaderTitleStyle).
const (
	titleStylePlain     = "plain"
	titleStyleSpine     = "spine"
	titleStyleBracketed = "bracketed"
	titleStyleLabel     = "label"
)

// renderHeader paints the filled hero bar + tagline. Edge-to-edge at cardWidth.
func (r renderer) renderHeader() string {
	h := r.cfg.Header
	surface := r.st.headerBg
	title := r.headerTitle(surface, h.TitleStyle)

	status := func(label string) string {
		if h.StatusText == "" && label == "" {
			return ""
		}
		out := ""
		if label != "" {
			out += inline(r.st.headerDim).Render(label + "  ")
		}
		if h.StatusText != "" {
			out += inline(r.st.headerAccent).Render("● ") +
				inline(r.st.headerMuted).Render(h.StatusText)
		}
		return out
	}

	contentWidth := r.cardWidth - headerRightPad
	info := status(h.StatusLabel)
	if info != "" && lipgloss.Width(title)+2+lipgloss.Width(info) > contentWidth {
		info = status(h.StatusLabelCompact)
	}

	var row string
	if info == "" {
		row = r.fillLine(title, r.cardWidth, surface)
	} else {
		gap := max(contentWidth-lipgloss.Width(title)-lipgloss.Width(info), 1)
		row = r.fillLine(
			title+
				r.st.fill(surface).Render(strings.Repeat(" ", gap))+
				info+
				r.st.fill(surface).Render(strings.Repeat(" ", headerRightPad)),
			r.cardWidth,
			surface,
		)
	}

	pad := r.st.headerFill.Width(r.cardWidth).Render("")
	parts := []string{pad, row, pad}

	if h.Tagline != "" {
		taglineRow := r.fillLine(
			inline(r.st.dim).Render(h.Tagline),
			r.cardWidth,
			r.st.blockBg,
		)
		parts = append(parts, r.blankLine(), taglineRow)
	}
	return join(parts...)
}

// headerTitle renders a wordmark whose background is exactly the header surface.
func (r renderer) headerTitle(surface color.Color, style string) string {
	name := r.cfg.Project
	switch strings.ToLower(style) {
	case titleStylePlain:
		return inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Accent))).Bold(true)).
			Render("  " + name + "  ")
	case titleStyleBracketed:
		return inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Dim)))).Render("  [ ") +
			inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Accent))).Bold(true)).Render(name) +
			inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Dim)))).Render(" ]  ")
	case titleStyleLabel:
		return inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Dim)))).Render("  devshell / ") +
			inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Fg))).Bold(true)).Render(name) +
			inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Dim)))).Render("  ")
	default: // spine
		return inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Accent)))).Render("  ▌ ") +
			inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Fg))).Bold(true)).Render(name) +
			inline(r.st.on(surface, r.st.h.Color(string(r.st.pal.Dim)))).Render("  ")
	}
}
