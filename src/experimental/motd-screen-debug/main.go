// motd-screen-debug is a throwaway terminal diagnostic. Delete it once the
// window-background behavior is understood.
//
// It runs the real `motd` binary as a subprocess (so measurement, rendering,
// and ColorWriter profile downgrade all happen through the production path),
// then overlays a diagnostic HUD using the same measurement chain
// (term.GetSize → COLUMNS → 80) and the same shared.ColorWriter.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"

	"golang.org/x/term"
)

func main() {
	motdBin := flag.String("motd", "motd", "MOTD executable to run (real binary, real config)")
	flag.Parse()

	bin, err := exec.LookPath(*motdBin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "motd-screen-debug:", err)
		os.Exit(1)
	}

	// Run the real motd — it inherits the TTY, measures the terminal, renders
	// through newRenderer + windowFill, and writes through shared.ColorWriter.
	// This is exactly what the dogfood shell does.
	cmd := exec.Command(bin)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "motd-screen-debug: motd failed:", err)
		os.Exit(1)
	}

	// Measure the terminal the same way systemRenderTerminal.Width() does:
	// term.GetSize on stdout, then COLUMNS env, then 80. We also capture
	// height from the same GetSize call (production discards it; we need it
	// for the explicit bottom-row paint).
	cols, rows, detected := measure(os.Stdout)
	stderrCols, stderrRows, stderrDetected := measure(os.Stderr)
	ttyCols, ttyRows, ttyDetected := measureTTY()

	if !detected {
		fmt.Fprintln(os.Stderr, "motd-screen-debug: no terminal dimensions detected on stdout")
	}

	paintOverlay(detected, cols, rows, bin, sizeReport{
		{"stdout", detected, cols, rows},
		{"stderr", stderrDetected, stderrCols, stderrRows},
		{"/dev/tty", ttyDetected, ttyCols, ttyRows},
	})
}

type sizeEntry struct {
	name     string
	detected bool
	columns  int
	rows     int
}

type sizeReport []sizeEntry

func paintOverlay(detected bool, cols, rows int, binPath string, report sizeReport) {
	// Write the overlay through shared.ColorWriter so color profile downgrade
	// matches the motd binary's output path.
	out := shared.ColorWriter(os.Stdout, os.Environ(), "")

	red := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5050"))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("#dcdcdc"))
	purple := lipgloss.NewStyle().Background(lipgloss.Color("#b428c8")).Foreground(lipgloss.Color("#ffffff"))

	var b strings.Builder
	b.WriteString("\x1b7") // save cursor

	writeAt(&b, 1, 1, red, clip("XXX  MOTD SCREEN DEBUG — top row reached", cols))
	writeAt(&b, 2, 1, muted, clip(fmt.Sprintf("selected=stdout  columns=%d  rows=%d  env COLUMNS=%q LINES=%q", cols, rows, os.Getenv("COLUMNS"), os.Getenv("LINES")), cols))
	writeAt(&b, 3, 1, muted, clip("motd="+binPath, cols))
	for i, e := range report {
		writeAt(&b, 4+i, 1, muted, clip(formatSize(e), cols))
	}

	instructionRow := min(8, rows)
	writeAt(&b, instructionRow, 1, red, clip("If only the purple bottom row is painted, SGR + Erase Display failed in this terminal.", cols))

	// Explicit cursor-addressed full-width row at the bottom of the detected
	// viewport. Painted with a distinct background via lipgloss — not via the
	// erase — so it's a direct BCE-vs-explicit-row comparison.
	bottom := rows
	writeAt(&b, bottom, 1, purple, clip("EXPLICIT ROW PAINT  "+strings.Repeat("X", cols), cols))

	b.WriteString("\x1b8") // restore cursor
	_, _ = fmt.Fprint(out, b.String())
}

func writeAt(out *strings.Builder, row, column int, style lipgloss.Style, text string) {
	fmt.Fprintf(out, "\x1b[%d;%dH", row, column)
	out.WriteString(style.Render(text))
}

func formatSize(e sizeEntry) string {
	if e.detected {
		return fmt.Sprintf("%s: columns=%d rows=%d", e.name, e.columns, e.rows)
	}
	return fmt.Sprintf("%s: size unavailable", e.name)
}

func clip(text string, width int) string {
	if width <= 0 {
		return text
	}
	if len(text) <= width {
		return text
	}
	return text[:width]
}

// measure mirrors systemRenderTerminal.Width() exactly: term.GetSize on the
// file descriptor, then COLUMNS env, then 80. Height comes from the same
// GetSize call (production discards it; we need it for the explicit row).
func measure(file *os.File) (cols, rows int, ok bool) {
	if c, r, err := term.GetSize(int(file.Fd())); err == nil && c > 0 {
		return c, r, true
	}
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		l, _ := strconv.Atoi(os.Getenv("LINES"))
		if l <= 0 {
			l = 24
		}
		return c, l, true
	}
	return 80, 24, false
}

func measureTTY() (cols, rows int, ok bool) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return 80, 24, false
	}
	defer tty.Close()
	return measure(tty)
}
