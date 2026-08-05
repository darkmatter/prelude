package main

import (
	"fmt"
	"strings"
)

// pinMode is the cycled pinned panel above the shell.
type pinMode int

const (
	pinOff pinMode = iota
	pinMotd
	pinMenu
	pinDocs
)

func (m pinMode) next() pinMode {
	return (m + 1) % 4
}

func (m pinMode) label() string {
	switch m {
	case pinOff:
		return "off"
	case pinMotd:
		return "motd"
	case pinMenu:
		return "menu"
	case pinDocs:
		return "docs"
	default:
		return "?"
	}
}

// layoutGeom is host-terminal geometry for pin + shell + status.
//
//	row 1 .. pinH          pin panel (0 when pin off)
//	row pinH+1 .. pinH+shellH   shell scroll region / PTY
//	row totalH             status line
type layoutGeom struct {
	cols, totalH int
	shellRows    int // configured shell I/O height when pin is on
	pin          pinMode

	pinH      int // pin panel height
	shellH    int // PTY / scroll-region height
	shellTop  int // 1-based first shell row
	statusRow int // 1-based status row (== totalH)
}

func computeLayout(cols, totalH, shellRows int, pin pinMode) layoutGeom {
	if totalH < 3 {
		totalH = 3
	}
	if cols < 1 {
		cols = 1
	}
	if shellRows < 1 {
		shellRows = 1
	}
	g := layoutGeom{
		cols:      cols,
		totalH:    totalH,
		shellRows: shellRows,
		pin:       pin,
		statusRow: totalH,
	}
	// Always keep 1 status row.
	avail := totalH - 1
	if pin == pinOff {
		g.pinH = 0
		g.shellH = avail
	} else {
		// Shell keeps min(shellRows, avail-1) so pin always has ≥1 row when on.
		maxShell := avail - 1
		if maxShell < 1 {
			maxShell = 1
		}
		g.shellH = shellRows
		if g.shellH > maxShell {
			g.shellH = maxShell
		}
		g.pinH = avail - g.shellH
	}
	g.shellTop = g.pinH + 1
	return g
}

func (g layoutGeom) statusLine() string {
	// Ctrl+G cycles pin (ASCII BEL is 0x07; we intercept before the PTY).
	return fitStatus(fmt.Sprintf(
		" spike  ·  pin:%s  ·  shell:%drow  ·  ^G cycle motd/menu/docs  ·  exit shell to leave ",
		g.pin.label(), g.shellH,
	), g.cols)
}

// clipANSILines is a crude clip: split on \n, take first max lines, pad.
// Does not understand CSI cursor moves — good enough for pinned snapshots.
func clipANSILines(s string, maxLines, cols int) string {
	if maxLines <= 0 {
		return ""
	}
	// Normalize
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	for len(lines) < maxLines {
		lines = append(lines, "")
	}
	// Soft-trim each line's visible width is hard with ANSI; pad plain blanks only.
	for i := range lines {
		if lines[i] == "" {
			lines[i] = strings.Repeat(" ", max(cols, 0))
		}
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
