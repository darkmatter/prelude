// prelude-motd renders the static devshell greeting from normalized JSON.
// Nix owns configuration; this command owns terminal probing and presentation.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"strconv"

	"prelude/shared"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// defaultConfigPath is injected by Nix at link time. Keeping configuration in
// a data file preserves one Go renderer without reintroducing a shell wrapper.
var defaultConfigPath string

func main() {
	configPathDefault := os.Getenv("PRELUDE_MOTD_CONFIG")
	if configPathDefault == "" {
		configPathDefault = defaultConfigPath
	}
	configPath := flag.String("config", configPathDefault, "path to the MOTD config JSON")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}

	if needsRelativeBackgrounds(cfg) || needsTerminalBackground(cfg) {
		var terminalBackground color.Color
		if needsTerminalBackground(cfg) {
			// stderr remains attached to the TTY when stdout is wrapped for color
			// profile conversion. Lip Gloss handles OSC querying and timeout behavior.
			var bgErr error
				terminalBackground, bgErr = queryTerminalBackground()
				if os.Getenv("PRELUDE_MOTD_DEBUG") != "" {
				if bgErr != nil {
					fmt.Fprintln(os.Stderr, "motd: background query failed:", bgErr)
				} else {
					fmt.Fprintln(os.Stderr, "motd: terminal background =", colorHex(terminalBackground))
				}
			}
		}
		cfg = resolveRelativeBackgrounds(cfg, terminalBackground)
	}

	rt := systemRuntime{}
	// Live status badges: spinner on stderr while checks run, then final MOTD.
	resolveHeaderStatuses(&cfg, rt)

	output := shared.ColorWriter(os.Stdout, os.Environ(), cfg.ColorProfile)
	if _, err := fmt.Fprint(output, render(cfg, terminalWidth(os.Stdout), rt)); err != nil {
		fmt.Fprintln(os.Stderr, "motd:", err)
		os.Exit(1)
	}
}

// queryTerminalBackground asks the terminal for its background color (OSC 11).
// Lip Gloss needs both fds to be TTYs, which fails when stdin or stdout is
// redirected (direnv, pipes, `nix develop -c`), so fall back to the
// controlling terminal directly.
func queryTerminalBackground() (color.Color, error) {
	bg, err := lipgloss.BackgroundColor(os.Stdin, os.Stderr)
	if err == nil {
		return bg, nil
	}
	tty, ttyErr := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if ttyErr != nil {
		return nil, err
	}
	defer tty.Close()
	return lipgloss.BackgroundColor(tty, tty)
}

func terminalWidth(output *os.File) int {
	if width, _, err := term.GetSize(int(output.Fd())); err == nil && width > 0 {
		return width
	}
	if width, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && width > 0 {
		return width
	}
	return 80
}
