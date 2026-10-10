package main

import (
	"fmt"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

type keyHint struct{ key, text string }

var navigationKeys = []keyHint{{"LOCKED", ""}, {"Ctrl+P", "unlock"}}
var leaderKeys = []keyHint{
	{"UNLOCKED", ""}, {"m", "motd"}, {"x", "menu"}, {"d", "docs"}, {"t", "window"},
	{"Tab", "focus"}, {"v", "layout"}, {"c", "close"}, {"Esc", "lock"}, {"Ctrl+P", "lock"},
}
var completionKeys = []keyHint{{"Tab", "next"}, {"Shift+Tab", "prev"}, {"Enter", "accept"}, {"Esc", "dismiss"}}

var (
	baseFooter     = keyHintsText(navigationKeys)
	leaderHints    = keyHintsText(leaderKeys)
	completionHint = strings.TrimSpace(keyHintsText(completionKeys))
)

// Match KeyHintsFooter's one-row keymap: padded accent keycaps, muted labels,
// and two spaces between groups. The host keeps its existing single-row geometry.
func keyHintsText(hints []keyHint) string {
	if len(hints) == 0 {
		return ""
	}
	var text strings.Builder
	text.WriteString("  ")
	for i, hint := range hints {
		if i > 0 {
			text.WriteString("  ")
		}
		text.WriteString(" " + hint.key + " ")
		if hint.text != "" {
			text.WriteString(" " + hint.text)
		}
	}
	return text.String()
}

func (h *host) footerContent() ([]keyHint, string) {
	if h.err != nil {
		return nil, "error: " + h.err.Error()
	}
	if h.leader {
		return leaderKeys, ""
	}
	if h.completionVisible() {
		return completionKeys, ""
	}
	hints := navigationKeys
	var status string
	if h.surface != surfaceNone {
		if !h.paneVisible {
			status = fmt.Sprintf("%s hidden | %s", h.surface, h.paneStatus())
		} else {
			status = h.paneLabel()
			if h.layout().Body.Empty() {
				status += " (hidden: resize to restore)"
			}
		}
	}
	if h.pendingCommand != "" {
		status = strings.TrimPrefix(status+" | command queued", " | ")
	}
	if h.notice != "" {
		status = strings.TrimSuffix(h.notice+" | "+status, " | ")
	}
	return hints, displayText(status)
}

func (h *host) footerLayout() ([]keyHint, string, int) {
	hints, status := h.footerContent()
	status = ansi.Truncate(status, max(0, h.cols-4), "")
	statusX := h.cols - 2 - ansi.StringWidth(status)
	budget := max(0, h.cols-4)
	if status != "" {
		budget = max(0, statusX-4)
	}
	width, count := 0, 0
	for _, hint := range hints {
		next := width + ansi.StringWidth(hint.key) + 2
		if hint.text != "" {
			next += ansi.StringWidth(hint.text) + 1
		}
		if count > 0 {
			next += 2
		}
		if next > budget {
			break
		}
		width, count = next, count+1
	}
	return hints[:count], status, statusX
}

func (h *host) footer() string {
	hints, status, statusX := h.footerLayout()
	text := keyHintsText(hints)
	if status != "" {
		text += strings.Repeat(" ", max(0, statusX-ansi.StringWidth(text))) + status
	}
	return text
}

func (h *host) footerStyle() uv.Style {
	fg := h.palette.Muted
	switch {
	case h.err != nil || h.paneError != nil || (h.pane != nil && (h.pane.err != nil || h.pane.done && h.pane.exitCode != 0)):
		fg = h.palette.Error
	case h.notice != "" || h.pendingCommand != "":
		fg = h.palette.Warning
	case h.pane != nil && !h.pane.done:
		fg = h.palette.Success
	}
	return uv.Style{Fg: fg, Attrs: uv.AttrBold}
}

func (h *host) drawFooter(buffer *uv.ScreenBuffer, y int) {
	buffer.FillArea(&uv.EmptyCell, uv.Rect(0, y, h.cols, 1))
	hints, status, statusX := h.footerLayout()
	x := 2
	for i, hint := range hints {
		if i > 0 {
			x += 2
		}
		key := " " + hint.key + " "
		style := uv.Style{Fg: h.palette.Accent2, Bg: h.palette.Bg, Attrs: uv.AttrBold}
		if hint.key == "UNLOCKED" {
			style.Fg, style.Bg = h.palette.Bg, h.palette.Success
		} else if hint.key == "LOCKED" {
			style.Fg = h.palette.Muted
		}
		drawStyledText(buffer, key, x, y, h.cols-x, style)
		x += ansi.StringWidth(key)
		if hint.text != "" {
			label := " " + hint.text
			drawStyledText(buffer, label, x, y, h.cols-x, uv.Style{Fg: h.palette.Muted})
			x += ansi.StringWidth(label)
		}
	}
	if status != "" {
		drawStyledText(buffer, status, statusX, y, h.cols-statusX, h.footerStyle())
	}
}
