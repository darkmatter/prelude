package motd

import (
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// StatusItems is a React-style, one-component-per-file presentation of the motd
// status badges. It paints resolved status chips, optionally compact, with the
// dot color reflecting Level. The component is stateless and uses the resolved
// renderer context for MOTD-specific styles and layout.
type StatusItems struct {
	r renderer
}

// Render paints resolved status badges. compact drops labels and keeps only the
// indicator + status text. Dot colors use semantic palette roles: unresolved
// static items are informational; checks resolve to success, warning, or error.
func (x StatusItems) Render(items []HeaderStatus, compact bool) string {
	var parts []string
	for _, it := range items {
		label, status := it.Label, it.Status
		if label == "" && status == "" {
			continue
		}
		dot := x.dot(it.Level)
		var chip string
		if compact {
			if status != "" {
				chip = ui.Inline(dot).Render("● ") + ui.Inline(x.r.st.headerMuted).Render(status)
			} else if label != "" {
				chip = ui.Inline(dot).Render("● ") + ui.Inline(x.r.st.headerMuted).Render(label)
			}
		} else {
			if label != "" {
				chip += ui.Inline(x.r.st.headerMuted).Render(label)
				if status != "" {
					chip += ui.Inline(x.r.st.headerDim).Render("  ")
				}
			}
			if status != "" {
				chip += ui.Inline(dot).Render("● ") + ui.Inline(x.r.st.headerMuted).Render(status)
			} else if label != "" {
				chip += ui.Inline(x.r.st.headerDim).Render("  ") + ui.Inline(dot).Render("●")
			}
		}
		if chip != "" {
			parts = append(parts, chip)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	sep := ui.Inline(x.r.st.headerDim).Render(" / ")
	result := strings.Join(parts, sep)
	if age := x.r.cfg.StatusAge; age != "" {
		result += ui.Inline(x.r.st.headerDim).Render("  ") + ui.Inline(x.r.st.headerMuted).Render(age)
	}
	return result
}

// dot keeps status semantics independent from decorative accent colors.
func (x StatusItems) dot(level string) lipgloss.Style {
	switch level {
	case "success":
		return x.r.st.headerSuccess
	case "warning":
		return x.r.st.headerWarning
	case "error":
		return x.r.st.headerError
	default:
		return x.r.st.headerInfo
	}
}

// Hint renders the asynchronous refresh hint plus any configured hint links
// (e.g. the repository), joined by the standard dot separator. Links render
// even when no async hint exists so a configured link always surfaces.
func (x StatusItems) Hint(header bool) string {
	style := x.r.st.dim
	linkContext := x.r.blockUI
	if header {
		style = x.r.st.headerDim
		linkContext = x.r.headerUI
	}
	var parts []string
	if x.r.cfg.StatusHint != "" {
		parts = append(parts, ui.Inline(style).Render(x.r.cfg.StatusHint))
	}
	for _, link := range x.r.cfg.Header.StatusHintLinks {
		if rendered := (ui.Link{Context: linkContext, Label: link.Label, URL: link.URL}).Render(); rendered != "" {
			parts = append(parts, rendered)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ui.Inline(style).Render("  ·  "))
}

// InlineHint renders a fixed-width status row with lights flush left and the
// asynchronous refresh hint flush right. It compacts the lights when needed and
// returns an empty string when even the compact row cannot fit, allowing callers
// to retain the stacked layout as a narrow-terminal fallback.
func (x StatusItems) InlineHint(items []HeaderStatus, width int, header bool) string {
	hint := x.Hint(header)
	status := x.Render(items, false)
	if hint == "" || status == "" || width <= 0 {
		return ""
	}
	if lipgloss.Width(status)+1+lipgloss.Width(hint) > width {
		status = x.Render(items, true)
	}
	gap := width - lipgloss.Width(status) - lipgloss.Width(hint)
	if gap < 1 {
		return ""
	}
	fill := x.r.st.blockFill
	if header {
		fill = x.r.st.headerFill
	}
	spacer := lipgloss.PlaceHorizontal(
		gap,
		lipgloss.Left,
		"",
		lipgloss.WithWhitespaceStyle(fill),
	)
	return status + spacer + hint
}
