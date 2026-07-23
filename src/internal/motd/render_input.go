package motd

import "time"

// RenderInput is the single pure input to Render: Config plus Cache.
// TerminalWidth/Height override cache/defaults when positive (tests / pure inject).
type RenderInput struct {
	Config         Config
	Cache          Cache
	TerminalWidth  int
	TerminalHeight int
}

// Render produces the MOTD banner purely from RenderInput.
// Missing/stale cache yields sparse UI (P1); never fails for live data absence.
// Post-banner diagnostics are not emitted (D2).
func Render(in RenderInput) string {
	model := Resolve(in.Config, in.Cache, in.TerminalWidth, in.TerminalHeight, time.Now())
	return render(model)
}
