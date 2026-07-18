package menu

import (
	"fmt"
	"strings"
)

// --- args view ---

// viewArgs assembles the argument-entry panel: the title chrome, the prompt row,
// the framed arg body + live preview + error line (rendered by ArgsView), and the
// status footer. The chrome layers (title, prompt, status) belong to the root;
// the args-specific framed body, preview, and error line belong to ArgsView, which
// owns the arg-entry state (chips, chipFocus, argErr, argTask). The Prompt owns
// the arg input string and is passed through to ArgsView.View for the live preview.
func (m model) viewArgs() string {
	t := *m.args.Task()
	title := fmt.Sprintf("%s %s — enter arguments", m.cfg.Project, t.displayName())
	argsBody := m.args.View(m.prompt.Value(), m.frame, m.listHeight()-3)
	return strings.Join([]string{
		m.title.View(title),
		m.prompt.View(),
		argsBody,
		m.status.View([][2]string{
			{"⇥", "chips"}, {"↵", "run"}, {"esc", "back"},
		}, "◆ args"),
	}, "\n")
}
