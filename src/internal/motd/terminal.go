package motd

import (
	"fmt"
	"image/color"
	"os"
	"strconv"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

type systemRenderTerminal struct {
	output *os.File
}

func (t systemRenderTerminal) Width() int {
	width, _ := t.size()
	return width
}

func (t systemRenderTerminal) Height() int {
	_, height := t.size()
	return height
}

func (t systemRenderTerminal) size() (int, int) {
	if width, height, err := term.GetSize(int(t.output.Fd())); err == nil && width > 0 {
		return width, max(height, 1)
	}
	if width, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && width > 0 {
		height, _ := strconv.Atoi(os.Getenv("LINES"))
		return width, max(height, 1)
	}
	return 80, 24
}

func (systemRenderTerminal) Background() (color.Color, error) {
	// stderr remains attached to the TTY when stdout is wrapped for color
	// profile conversion. Lip Gloss handles OSC querying and timeout behavior.
	bg, err := lipgloss.BackgroundColor(os.Stdin, os.Stderr)
	if err == nil {
		return bg, nil
	}

	// Lip Gloss needs both fds to be TTYs, which fails when stdin or stdout is
	// redirected (direnv, pipes, `nix develop -c`), so use the controlling TTY.
	tty, ttyErr := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if ttyErr != nil {
		return nil, err
	}
	defer tty.Close()
	return lipgloss.BackgroundColor(tty, tty)
}

func (systemRenderTerminal) Debug() bool {
	return os.Getenv("PRELUDE_MOTD_DEBUG") != ""
}

func (systemRenderTerminal) Diagnostic(message string) {
	fmt.Fprintln(os.Stderr, message)
}
