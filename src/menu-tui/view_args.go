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
	body = append(body, m.paint(
		st.sp.Render(strings.Repeat(" ", padX))+st.sMuted.Render(letterSpace("arguments")),
		st.sp, inner,
	))
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
			st.sAccent.Bold(true).Width(tokenW).Render(a.Token) + st.sp.Render("  ") +
			tagStyle.Width(8).Render(tag) + st.sp.Render("  ") +
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
					// Focused options use the phosphor selection treatment.
					chips = append(chips, st.selText.Bold(true).Render(label))
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

	// Pad the body to a stable height before the open preview.
	h := m.listHeight() - 3
	for len(body) < h {
		body = append(body, m.blank(inner))
	}
	if len(body) > h {
		body = body[:h]
	}

	// Live preview uses the same assembly path as final submission; open
	// full-width region under the frame (no side rails).
	argumentLine := strings.TrimSpace(m.input.Value())
	preview := st.openSp.Render(strings.Repeat(" ", padX)) +
		st.openAccent.Render("$ ") +
		st.openFg.Render(assembleInvocation(*t, argumentLine))
	if argumentLine == "" {
		preview += st.openDim.Render(" …")
	}
	openPreview := st.openSp.Width(inner + 2).MaxWidth(inner + 2).Render(preview)

	var errLine string
	if m.argErr != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(st.pal.Error))).
			Background(st.openColor)
		errLine = st.openSp.Width(inner+2).MaxWidth(inner+2).Render(
			st.openSp.Render(strings.Repeat(" ", padX)) + errStyle.Render(m.argErr),
		)
	}

	parts := []string{
		m.mutedTitleRow(title, inner),
		m.promptLine(inner, t.Name),
		m.frameTop(inner),
	}
	parts = append(parts, body...)
	parts = append(parts, m.frameBottom(inner), openPreview)
	if errLine != "" {
		parts = append(parts, errLine)
	}
	parts = append(parts,
		m.statusBar([][2]string{
			{"⇥", "chips"}, {"↵", "run"}, {"esc", "back"},
		}, "◆ args", inner),
	)
	return strings.Join(parts, "\n")
}
