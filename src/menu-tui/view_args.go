package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// --- args view ---------------------------------------------------------------

func (m model) viewArgs(inner int) string {
	st := m.st
	t := m.argTask
	title := fmt.Sprintf("%s %s — enter arguments", m.cfg.Project, t.Name)

	var body []string
	body = append(body, m.blank(inner))
	body = append(body, m.paint(st.sp.Render(strings.Repeat(" ", padX))+st.sMuted.Render(letterSpace("arguments")), st.sp, inner))
	body = append(body, m.blank(inner))

	tokenW := 4
	for _, a := range t.Args {
		tokenW = max(tokenW, lipgloss.Width(a.Token))
	}

	chipIdx := 0
	for _, a := range t.Args {
		tag := "OPTIONAL"
		tagStyle := st.sDim
		switch {
		case a.Required:
			tag, tagStyle = "REQUIRED", st.sErr
		case a.Boolean:
			tag = "FLAG"
		}
		row := st.sp.Render(strings.Repeat(" ", padX)) +
			st.sAccent2.Render(padRight(a.Token, tokenW)) + st.sp.Render("  ") +
			tagStyle.Render(padRight(tag, 8)) + st.sp.Render("  ") +
			st.sMuted.Render(a.Description)
		body = append(body, m.paint(row, st.sp, inner))

		nChips := len(a.Options)
		if a.Boolean && nChips == 0 {
			nChips = 1
		}
		if nChips > 0 {
			var chips []string
			for i := 0; i < nChips; i++ {
				c := m.chips[chipIdx]
				label := " " + c.label + " "
				if chipIdx == m.chipFocus {
					chips = append(chips, st.selText.Render(label))
				} else {
					chips = append(chips, st.optChip.Render(label))
				}
				chipIdx++
			}
			row := st.sp.Render(strings.Repeat(" ", padX+tokenW+2)) +
				strings.Join(chips, st.sp.Render(" "))
			body = append(body, m.paint(row, st.sp, inner))
		}
		body = append(body, m.blank(inner))
	}

	// Pad the body to a stable height before the preview.
	h := m.listHeight() - 3
	for len(body) < h {
		body = append(body, m.blank(inner))
	}
	if len(body) > h {
		body = body[:h]
	}

	// Live preview of the assembled command.
	preview := st.sp.Render(strings.Repeat(" ", padX)) + st.sAccent.Render("$ ") + st.sFg.Render(t.Run)
	if v := strings.TrimSpace(m.input.Value()); v != "" {
		preview += st.sFg.Render(" " + v)
	} else {
		preview += st.sDim.Render(" …")
	}

	tail := []string{m.frameDiv(inner), m.paint(preview, st.sp, inner)}
	if m.argErr != "" {
		tail = append(tail, m.paint(st.sp.Render(strings.Repeat(" ", padX))+st.sErr.Render(m.argErr), st.sp, inner))
	} else {
		tail = append(tail, m.blank(inner))
	}

	parts := []string{
		m.frameTop(inner),
		m.titleBar(title, inner),
		m.frameDiv(inner),
		m.promptLine(inner, t.Name),
		m.frameDiv(inner),
	}
	parts = append(parts, body...)
	parts = append(parts, tail...)
	parts = append(parts,
		m.frameDiv(inner),
		m.statusBar([][2]string{
			{"⇥", "chips"}, {"↵", "run"}, {"esc", "back"},
		}, m.st.bar(m.st.pal.Accent2).Render("◆ args"), inner),
		m.frameBottom(inner),
	)
	return strings.Join(parts, "\n")
}
