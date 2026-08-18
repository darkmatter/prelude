package menu

import (
	"fmt"
	"strings"
)

// --- args view ---

// viewArgs assembles the argument-entry panel: the title chrome, the prompt
// row, the framed arg body + error line (rendered by ArgsView), and the
// status footer. The chrome layers (title, prompt, status) belong to the root;
// the args-specific framed body and error line belong to ArgsView, which owns
// the arg-entry state (chips, chipFocus, argErr, argTask). The live
// invocation preview is the root's last-row overlay (renderScriptPreviewRow in
// view.go), not a second in-panel copy, so the framed body reclaims the row
// the old openPreview occupied: bodyHeight is listHeight-2 (frame bottom cap
// + one row of breathing room below the last arg).
func (m model) viewArgs() string {
	t := *m.args.Task()
	title := fmt.Sprintf("%s %s — enter arguments", m.cfg.Project, t.displayName())
	argsBody := m.args.View(m.frame, m.layout.listHeight-2)
	return strings.Join([]string{
		m.title.View(title),
		m.prompt.View(m.promptCtx, m.promptPlaceholder),
		argsBody,
		m.status.View([][2]string{
			{"⇥", "chips"}, {"↵", "run"}, {"esc", "back"},
		}, "◆ args"),
	}, "\n")
}
