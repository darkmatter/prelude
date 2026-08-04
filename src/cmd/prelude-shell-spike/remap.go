package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// remapShellOutput rewrites complete escape sequences from the child PTY so
// absolute positions land in the shell strip, not the pin panel.
//
// DECSTBM alone is not enough: CUP/ED are absolute on the host. Without remap,
// starship's ESC[1;1H / ESC[2J paints into pin rows 1..pinH and leaves them blank.
func remapShellOutput(p []byte, shellTop, shellH, cols int) []byte {
	if shellH < 1 {
		shellH = 1
	}
	if shellTop < 1 {
		return p
	}
	var out bytes.Buffer
	i := 0
	for i < len(p) {
		if p[i] != 0x1b || i+1 >= len(p) {
			out.WriteByte(p[i])
			i++
			continue
		}
		if p[i+1] == '[' {
			j := i + 2
			for j < len(p) && !(p[j] >= 0x40 && p[j] <= 0x7e) {
				j++
			}
			if j >= len(p) {
				out.Write(p[i:])
				break
			}
			rewritten := remapCSI(p[i:j+1], shellTop, shellH, cols)
			if len(rewritten) > 0 {
				out.Write(rewritten)
			}
			i = j + 1
			continue
		}
		// ESC c (RIS) — soft-reset would wreck host chrome; drop it.
		if p[i+1] == 'c' {
			i += 2
			continue
		}
		// Two-byte ESC (including ESC 7/8): pass through.
		if i+2 <= len(p) {
			out.Write(p[i : i+2])
			i += 2
			continue
		}
		out.WriteByte(p[i])
		i++
	}
	return out.Bytes()
}

func remapCSI(seq []byte, shellTop, shellH, cols int) []byte {
	if len(seq) < 3 {
		return seq
	}
	final := seq[len(seq)-1]
	params := string(seq[2 : len(seq)-1])

	// Private-mode CSI: ESC [ ? ... h/l  (alt screen, origin mode, etc.)
	if strings.HasPrefix(params, "?") {
		if final == 'h' || final == 'l' {
			return remapPrivateModes(params, final)
		}
		return seq
	}

	switch final {
	case 'H', 'f': // CUP / HVP
		row, col := 1, 1
		if params != "" {
			parts := strings.Split(params, ";")
			if parts[0] != "" {
				if n, err := strconv.Atoi(parts[0]); err == nil && n > 0 {
					row = n
				}
			}
			if len(parts) > 1 && parts[1] != "" {
				if n, err := strconv.Atoi(parts[1]); err == nil && n > 0 {
					col = n
				}
			}
		}
		row = clamp(row, 1, shellH) + shellTop - 1
		return []byte(fmt.Sprintf("\033[%d;%d%c", row, col, final))

	case 'd': // VPA
		row := 1
		if params != "" {
			if n, err := strconv.Atoi(strings.TrimPrefix(params, ";")); err == nil && n > 0 {
				row = n
			}
		}
		row = clamp(row, 1, shellH) + shellTop - 1
		return []byte(fmt.Sprintf("\033[%dd", row))

	case 'r': // DECSTBM from child — host owns the region
		return nil

	case 'J': // ED — never full-screen clear into the pin
		return clearShellRegionCSI(shellTop, shellH, cols)

	default:
		return seq
	}
}

func clearShellRegionCSI(shellTop, shellH, cols int) []byte {
	var b strings.Builder
	b.WriteString("\0337")
	if cols < 1 {
		cols = 1
	}
	blank := strings.Repeat(" ", cols)
	for row := shellTop; row < shellTop+shellH; row++ {
		fmt.Fprintf(&b, "\033[%d;1H\033[2K%s", row, blank)
	}
	b.WriteString("\0338")
	return []byte(b.String())
}

// remapPrivateModes strips modes that break host chrome:
//   - 6     DECOM (origin mode): CUP becomes relative to scroll region → pin paints into shell
//   - 47/1047/1049  alt screen buffers: would hide pin/status entirely
func remapPrivateModes(params string, final byte) []byte {
	body := strings.TrimPrefix(params, "?")
	if body == "" {
		return nil
	}
	var keep []string
	for _, p := range strings.Split(body, ";") {
		switch p {
		case "6", "47", "1047", "1049":
			continue
		case "":
			continue
		default:
			keep = append(keep, p)
		}
	}
	if len(keep) == 0 {
		return nil
	}
	return []byte(fmt.Sprintf("\033[?%s%c", strings.Join(keep, ";"), final))
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
