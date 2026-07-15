package main

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// renderSessionTerminal isolates terminal state and diagnostics from the ordered
// preparation required for one render pass.
type renderSessionTerminal interface {
	Width() int
	Background() (color.Color, error)
	Debug() bool
	Diagnostic(message string)
}

// renderSession prepares terminal-dependent configuration and live statuses
// before producing the final MOTD. Runtime probes remain lazy inside render.
func renderSession(cfg Config, runtime Runtime, terminal renderSessionTerminal) string {
	terminalWidth := terminal.Width()

	if needsRelativeBackgrounds(cfg) || needsTerminalBackground(cfg) {
		var terminalBackground color.Color
		if needsTerminalBackground(cfg) {
			var err error
			terminalBackground, err = terminal.Background()
			if terminal.Debug() {
				if err != nil {
					terminal.Diagnostic("motd: background query failed: " + err.Error())
				} else {
					terminal.Diagnostic("motd: terminal background = " + colorHex(terminalBackground))
				}
			}
		}
		cfg = resolveRelativeBackgrounds(cfg, terminalBackground)
	}

	diagnostics := resolveHeaderStatuses(&cfg, runtime)
	output := render(cfg, terminalWidth, runtime)
	if len(diagnostics) == 0 {
		return output
	}
	return output + strings.Join(diagnostics, "\n") + "\n"
}

type systemRenderTerminal struct {
	output *os.File
}

func (t systemRenderTerminal) Width() int {
	if width, _, err := term.GetSize(int(t.output.Fd())); err == nil && width > 0 {
		return width
	}
	if width, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && width > 0 {
		return width
	}
	return 80
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
