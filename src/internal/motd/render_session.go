package motd

import (
	"image/color"
	"strings"
)

// renderSessionTerminal isolates terminal state and diagnostics from the ordered
// preparation required for one render pass.
type renderSessionTerminal interface {
	Width() int
	Height() int
	Background() (color.Color, error)
	Debug() bool
	Diagnostic(message string)
}

// renderSession prepares terminal-dependent configuration and live statuses
// before producing the final MOTD. Runtime probes remain lazy inside render.
func renderSession(cfg Config, runtime Runtime, terminal renderSessionTerminal, store statusCacheStore) string {
	terminalWidth := terminal.Width()
	terminalHeight := terminal.Height()

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
			cfg = resolveRelativeBackgrounds(cfg, terminalBackground)
		}
	}

	diagnostics := applyAsyncCache(&cfg, store)
	diagnostics = append(diagnostics, (StatusResolver{runtime: runtime}).Resolve(&cfg)...)
	output := render(cfg, terminalWidth, terminalHeight, runtime)
	if len(diagnostics) == 0 {
		return output
	}
	return output + strings.Join(diagnostics, "\n") + "\n"
}
