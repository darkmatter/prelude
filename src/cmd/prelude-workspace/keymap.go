package main

import (
	"fmt"
	"image/color"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// keyHint is a keycap and its muted label. A badge joins the key and what it
// unlocks in one cap, so a single element is both state and shortcut; an
// active badge is filled.
type keyHint struct {
	key, text     string
	badge, active bool
}

var (
	lockedBadge   = keyHint{key: "^P", text: "layout", badge: true}
	unlockedBadge = keyHint{key: "^P", text: "layout", badge: true, active: true}
)

// Ctrl+P only unlocks layout commands, so the left side stays empty until a
// window is on screen to move, and closing the window locks. Alt+W and Alt+Tab
// cover show/hide and focus.
var (
	navigationKeys   = []keyHint{}
	windowKeys       = []keyHint{lockedBadge}
	leaderWindowKeys = []keyHint{unlockedBadge, layoutKeys, {key: "Esc", text: "lock"}}
)

// layoutKeys labels [ ] with the placement strip, so the keys and where they
// lead read as one hint. Its text stands in for width; drawFooter colors it.
var layoutKeys = keyHint{key: "[ ]", text: placementStrip()}

func placementStrip() string {
	glyphs := make([]string, len(placements))
	for i, where := range placements {
		glyphs[i] = where.glyph()
	}
	return strings.Join(glyphs, " ")
}

var completionKeys = []keyHint{{key: "Tab", text: "next"}, {key: "Shift+Tab", text: "prev"}, {key: "Enter", text: "accept"}, {key: "Esc", text: "dismiss"}}

var completionHint = strings.TrimSpace(keyHintsText(completionKeys))

func (k keyHint) keycap() string {
	if k.badge {
		return " " + k.key + " " + k.text + " "
	}
	return " " + k.key + " "
}

func (k keyHint) width() int {
	width := ansi.StringWidth(k.keycap())
	if !k.badge && k.text != "" {
		width += 1 + ansi.StringWidth(k.text)
	}
	return width
}

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
		text.WriteString(hint.keycap())
		if !hint.badge && hint.text != "" {
			text.WriteString(" " + hint.text)
		}
	}
	return text.String()
}

// statusSpan is one styled run of the right-aligned footer status. Text uses
// fg or muted; only the viewer pill paints a background, and a failure shows
// as a red element rather than red text.
type statusSpan struct {
	text    string
	fg, bg  color.Color
	bold    bool
	reverse bool
}

const (
	statusSeparator = " · "
	// The pill's keys are pressed with Option/Alt.
	optionPrefix = "⌥ + "
	// Powerline caps around the viewer pill, matching the generated prompt's
	// U+E0B0 dividers. The left cap faces the other way to close the pill.
	pillLeftCap  = "\ue0b2"
	pillDivider  = "\ue0b0"
	pillRightCap = "\ue0b0"
	// Thin dividers between the shell and viewer targets point at the
	// focused one.
	pillThinDivider     = "\ue0b1"
	pillThinLeftDivider = "\ue0b3"
)

// footerContent builds the hints and status. Unlocked, the layout keys and
// placement strip join the left hints; the status carries no strip.
func (h *host) footerContent() ([]keyHint, []statusSpan) {
	if h.err != nil {
		return nil, []statusSpan{
			{text: displayText("error: " + h.err.Error()), fg: h.palette.Fg},
			{text: " ●", fg: h.palette.Error},
		}
	}
	open := h.windowOpen()
	hints := navigationKeys
	switch {
	case h.leader && open:
		hints = leaderWindowKeys
	case h.completionVisible():
		return completionKeys, nil
	case open:
		hints = windowKeys
	}
	var groups [][]statusSpan
	if h.notice != "" {
		groups = append(groups, []statusSpan{{text: displayText(h.notice), fg: h.palette.Warning}})
	}
	if state := h.paneState(open); state != nil {
		groups = append(groups, state)
	}
	groups = append(groups, h.viewerPill(open))
	if h.pendingCommand != "" {
		groups = append(groups, []statusSpan{{text: "command queued", fg: h.palette.Warning}})
	}
	var spans []statusSpan
	for i, group := range groups {
		if i > 0 {
			// Pill caps already divide; a dot beside them would double up.
			separator := statusSeparator
			if spans[len(spans)-1].text == pillRightCap || group[0].text == optionPrefix {
				separator = " "
			}
			spans = append(spans, statusSpan{text: separator, fg: h.palette.Muted})
		}
		spans = append(spans, group...)
	}
	return hints, spans
}

func (h *host) paneFailed() bool {
	return h.paneError != nil || (h.pane != nil && (h.pane.err != nil || h.pane.done && h.pane.exitCode != 0))
}

// paneState words what the pill cannot: why the window stopped or why it is
// not drawn. Failures stay regular fg text; the pill's red segment marks them.
func (h *host) paneState(open bool) []statusSpan {
	switch {
	case h.paneError != nil || (h.pane != nil && h.pane.err != nil):
		return []statusSpan{{text: "error", fg: h.palette.Fg}}
	case h.pane != nil && h.pane.done:
		fg := h.palette.Fg
		if h.pane.exitCode == 0 {
			fg = h.palette.Muted
		}
		return []statusSpan{{text: fmt.Sprintf("exited %d", h.pane.exitCode), fg: fg}}
	case open && h.layout().Body.Empty():
		return []statusSpan{{text: "resize to restore", fg: h.palette.Warning}}
	}
	return nil
}

// windowOpen reports a viewer on screen: the only state with layout commands.
func (h *host) windowOpen() bool { return h.surface != surfaceNone && h.paneVisible }

type pillSegment struct {
	key, text     string
	fg, fill      color.Color
	bold, reverse bool
}

// viewerPill is a row of Option/Alt keys, each segment naming the key and what
// it cycles. Closed, it is one dim segment offering w to show the viewer. Open,
// w hides it and the accent fill marks whichever of viewer and shell has focus;
// ⇥ marks the one Alt+Tab reaches. The w key is bold accent in both states. A
// failed viewer turns its segment red.
func (h *host) viewerPill(open bool) []statusSpan {
	quiet := pillSegment{fg: h.palette.Muted, fill: h.palette.Surface}
	action := quiet
	action.key, action.text = "w", "viewer"
	segments := []*pillSegment{&action}
	prefix := h.palette.Muted
	keyFg := h.palette.Accent2
	pointer := pillThinDivider
	if open {
		name, shell := quiet, quiet
		name.text, shell.text = h.surface.String(), "shell"
		// Shown, the hide chip carries the fill; focus is told by the accent
		// foreground and the divider between the targets pointing at it.
		action.text, action.fg, action.fill, action.bold, action.reverse = "hide", h.palette.Bg, h.palette.Accent, true, h.palette.Accent == nil
		keyFg, prefix = h.palette.Bg, h.palette.Fg
		focused, other := &shell, &name
		if h.paneFocused() {
			focused, other, pointer = &name, &shell, pillThinLeftDivider
		}
		focused.fg, focused.bold = h.palette.Accent, true
		if h.paneInteractive() {
			other.text = "⇥ " + other.text
		}
		if h.paneFailed() {
			name.fg, name.fill, name.reverse = h.palette.Bg, h.palette.Error, h.palette.Error == nil
		}
		segments = append(segments, &name, &shell)
	} else if h.paneFailed() {
		action.fg = h.palette.Error // A hidden failure has no viewer chip to paint.
	}
	spans := []statusSpan{{text: optionPrefix, fg: prefix}, {text: pillLeftCap, fg: segments[0].fill}}
	for i, segment := range segments {
		if i > 0 {
			// Equally painted neighbours get a thin muted divider that
			// points at the focused target; otherwise the fill changes.
			prev := segments[i-1]
			divider := statusSpan{text: pillDivider, fg: prev.fill, bg: segment.fill}
			if prev.fill == segment.fill && prev.reverse == segment.reverse {
				divider.text, divider.fg = pointer, h.palette.Muted
			}
			spans = append(spans, divider)
		}
		text := " " + segment.text + " "
		if segment.key != "" {
			spans = append(spans, statusSpan{text: " " + segment.key, fg: keyFg, bg: segment.fill, bold: true, reverse: segment.reverse})
		}
		spans = append(spans, statusSpan{
			text: text, fg: segment.fg, bg: segment.fill,
			bold: segment.bold, reverse: segment.reverse,
		})
	}
	return append(spans, statusSpan{text: pillRightCap, fg: segments[len(segments)-1].fill})
}

func spansText(spans []statusSpan) string {
	var text strings.Builder
	for _, span := range spans {
		text.WriteString(span.text)
	}
	return text.String()
}

func (h *host) footerLayout() ([]keyHint, []statusSpan, string, int) {
	hints, spans, status, statusX, fit := h.fitFooter()
	return hints[:fit], spans, status, statusX
}

// fitFooter reserves the status first and returns how many hints fit before it.
func (h *host) fitFooter() ([]keyHint, []statusSpan, string, int, int) {
	hints, spans := h.footerContent()
	status := ansi.Truncate(spansText(spans), max(0, h.cols-4), "")
	statusX := h.cols - 2 - ansi.StringWidth(status)
	budget := max(0, h.cols-4)
	if status != "" {
		budget = max(0, statusX-4)
	}
	width, count := 0, 0
	for _, hint := range hints {
		next := width + hint.width()
		if count > 0 {
			next += 2
		}
		if next > budget {
			break
		}
		width, count = next, count+1
	}
	return hints, spans, status, statusX, count
}

func (h *host) footer() string {
	hints, _, status, statusX := h.footerLayout()
	text := keyHintsText(hints)
	if status != "" {
		text += strings.Repeat(" ", max(0, statusX-ansi.StringWidth(text))) + status
	}
	return text
}

func (h *host) drawFooter(buffer *uv.ScreenBuffer, y int) {
	buffer.FillArea(&uv.EmptyCell, uv.Rect(0, y, h.cols, 1))
	hints, spans, status, statusX := h.footerLayout()
	x := 2
	for i, hint := range hints {
		if i > 0 {
			x += 2
		}
		keycap := uv.Style{Fg: h.palette.Accent2, Bg: h.palette.Bg, Attrs: uv.AttrBold}
		if hint.badge {
			// The key keeps its keycap color; the mode reads muted until
			// unlocked, when the whole badge fills with accent.
			mode := uv.Style{Fg: h.palette.Muted, Bg: h.palette.Bg, Attrs: uv.AttrBold}
			if hint.active {
				keycap.Fg, keycap.Bg = h.palette.Bg, h.palette.Accent
				mode = keycap
			}
			key := " " + hint.key + " "
			drawStyledText(buffer, key, x, y, h.cols-x, keycap)
			x += ansi.StringWidth(key)
			label := hint.text + " "
			drawStyledText(buffer, label, x, y, h.cols-x, mode)
			x += ansi.StringWidth(label)
			continue
		}
		key := hint.keycap()
		drawStyledText(buffer, key, x, y, h.cols-x, keycap)
		x += ansi.StringWidth(key)
		if hint == layoutKeys {
			// The strip is the [ ] label: every layout, the current one lit.
			for _, where := range placements {
				style := uv.Style{Fg: h.palette.Muted}
				if where == h.placement {
					style = uv.Style{Fg: h.palette.Accent, Attrs: uv.AttrBold}
				}
				drawStyledText(buffer, " "+where.glyph(), x, y, h.cols-x, style)
				x += 1 + ansi.StringWidth(where.glyph())
			}
		} else if hint.text != "" {
			label := " " + hint.text
			drawStyledText(buffer, label, x, y, h.cols-x, uv.Style{Fg: h.palette.Muted})
			x += ansi.StringWidth(label)
		}
	}
	end := statusX + ansi.StringWidth(status)
	for x := statusX; len(spans) > 0 && x < end; spans = spans[1:] {
		style := uv.Style{Fg: spans[0].fg, Bg: spans[0].bg}
		if spans[0].bold {
			style.Attrs |= uv.AttrBold
		}
		if spans[0].reverse {
			style.Attrs |= uv.AttrReverse
		}
		drawStyledText(buffer, spans[0].text, x, y, end-x, style)
		x += ansi.StringWidth(spans[0].text)
	}
}
