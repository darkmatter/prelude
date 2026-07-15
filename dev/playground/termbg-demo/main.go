// Command termbg-demo verifies at runtime that the terminal background can be
// queried (OSC 11) and used to fade a painted window surface into the real
// terminal background.
//
// It renders a full-size container: a window-background plateau filling the
// terminal, with every edge (and corner) fading into whatever color the
// query returned. Diagnostics are printed in the center — including the raw
// OSC 11 response, so you can see whether your terminal reports alpha
// (rgba:...) or only an opaque base color (rgb:...).
//
// If detection works and your terminal is opaque, the container should melt
// into the screen with no visible seam. On translucent terminals the fade
// lands on the opaque base color, so the outermost cells will still sit
// slightly proud of the see-through background — that gap is the compositor
// alpha, which no escape sequence exposes.
package main

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

var (
	// Fallback surface when detection fails; when the query succeeds the
	// surface is derived at runtime by darkening the detected terminal color.
	windowBg = color.Color(lipgloss.Color("#1a1626"))
	fgC      = lipgloss.Color("#c8bfe0")
	dimC     = lipgloss.Color("#6f6590")
)

const surfaceDarken = 0.5 // window surface vs detected terminal bg

const (
	fadeX = 5  // horizontal fade depth, in columns
	fadeY = 2  // vertical fade depth, in rows
	rampN = 32 // gradient lookup resolution
)

func hexRGB(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// alphaOf reports the alpha channel of the parsed color. Terminals that
// answer OSC 11 with "rgba:RRRR/GGGG/BBBB/AAAA" surface it here; "rgb:"
// answers parse as opaque.
func alphaOf(c color.Color) float64 {
	_, _, _, a := c.RGBA()
	return float64(a) / 0xffff
}

// queryTerminalBackground mirrors motd's runtime strategy: stdin+stderr
// first (stdout may be redirected), then a direct /dev/tty handle.
func queryTerminalBackground() (color.Color, string, error) {
	if bg, err := lipgloss.BackgroundColor(os.Stdin, os.Stderr); err == nil {
		return bg, "OSC 11 via stdin/stderr", nil
	} else if tty, ttyErr := os.OpenFile("/dev/tty", os.O_RDWR, 0); ttyErr == nil {
		defer tty.Close()
		if bg, err2 := lipgloss.BackgroundColor(tty, tty); err2 == nil {
			return bg, "OSC 11 via /dev/tty", nil
		}
		return nil, "", fmt.Errorf("stdin/stderr: %v; /dev/tty: query failed", err)
	} else {
		return nil, "", fmt.Errorf("stdin/stderr: %v; /dev/tty: %v", err, ttyErr)
	}
}

// rawOSC11 does a manual OSC 11 roundtrip against /dev/tty and returns the
// raw response bytes, so we can see exactly what the terminal claims —
// including whether it uses the rgb: or rgba: form.
func rawOSC11() (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer tty.Close()
	fd := int(tty.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, old)

	if _, err := tty.WriteString("\x1b]11;?\x07"); err != nil {
		return "", err
	}
	tty.SetReadDeadline(time.Now().Add(2 * time.Second))
	var buf []byte
	b := make([]byte, 1)
	for {
		if _, err := tty.Read(b); err != nil {
			return string(buf), err
		}
		if b[0] == '\x07' { // BEL terminator
			break
		}
		if b[0] == '\\' && len(buf) > 0 && buf[len(buf)-1] == '\x1b' { // ST terminator
			buf = buf[:len(buf)-1]
			break
		}
		buf = append(buf, b[0])
		if len(buf) > 128 {
			break
		}
	}
	return strings.TrimPrefix(string(buf), "\x1b]11;"), nil
}

func main() {
	termW, termH := 80, 24
	if w, h, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && h > 0 {
		termW, termH = w, h
	}
	height := max(termH-1, 5) // leave the last row for the shell prompt

	termBg, how, err := queryTerminalBackground()
	raw, _ := rawOSC11()
	if err == nil {
		windowBg = lipgloss.Darken(termBg, surfaceDarken)
	}

	var info []string
	if err != nil {
		info = []string{
			"✗ terminal background NOT detected",
			err.Error(),
			"",
			"container rendered with hard edges",
		}
	} else {
		info = []string{
			fmt.Sprintf("✓ terminal bg  %s   (%s)", hexRGB(termBg), how),
			fmt.Sprintf("window surface %s (terminal bg darkened %d%%)", hexRGB(windowBg), int(surfaceDarken*100)),
			fmt.Sprintf("reported alpha %.2f", alphaOf(termBg)),
		}
		if raw != "" {
			info = append(info, fmt.Sprintf("raw response   %q", raw))
		}
		info = append(info, "", "every edge fades into the detected color")
	}

	// Gradient lookup ramp, terminal color → window surface.
	var ramp []color.Color
	if err == nil {
		ramp = lipgloss.Blend1D(rampN, termBg, windowBg)
	}

	// colorAt maps a cell to its background: distance to the nearest edge,
	// normalized by the fade depth per axis; the min of the two axes shapes
	// the corners. t=0 is exactly the terminal color, t>=1 is the plateau.
	colorAt := func(x, y int) color.Color {
		if ramp == nil {
			return windowBg
		}
		tx := float64(min(x, termW-1-x)) / fadeX
		ty := float64(min(y, height-1-y)) / fadeY
		t := min(tx, ty, 1)
		return ramp[int(t*float64(rampN-1))]
	}

	fadeRun := func(y, x0, x1 int) string {
		var b strings.Builder
		for x := x0; x < x1; x++ {
			b.WriteString(lipgloss.NewStyle().Background(colorAt(x, y)).Render(" "))
		}
		return b.String()
	}

	textTop := max((height-len(info))/2, 0)
	var out strings.Builder
	for y := range height {
		line := ""
		if i := y - textTop; i >= 0 && i < len(info) {
			line = info[i]
		}
		if line == "" {
			out.WriteString(fadeRun(y, 0, termW))
		} else {
			// Text rows sit in the vertical plateau (ty=1), so only the
			// horizontal fade applies; the middle is uniform window bg.
			style := lipgloss.NewStyle().Foreground(dimC)
			if i := y - textTop; i == 0 {
				style = style.Foreground(fgC).Bold(true)
			}
			inner := termW - 2*fadeX
			mid := style.Background(windowBg).
				Width(inner).MaxWidth(inner).Align(lipgloss.Center).Render(line)
			out.WriteString(fadeRun(y, 0, fadeX) + mid + fadeRun(y, termW-fadeX, termW))
		}
		if y < height-1 {
			out.WriteByte('\n')
		}
	}
	fmt.Println(out.String())
}
