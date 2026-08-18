package menu

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/ui"
)

// ArgsView is the argument-entry panel body: a self-contained bubbletea sub-model
// that owns the chip-selection, argument-error, and current-task state for arg
// mode. The root menu model holds an ArgsView and routes key events to its
// mutating methods; the Prompt owns the arg input string (its textinput); the
// root orchestrates between them (e.g., on enter with a chip focused, the root
// asks ArgsView for the focused chip, appends its token to the Prompt, then
// clears chip focus on ArgsView).
//
// One element per file (React-style): the args body is its own component, so
// argument-entry state is no longer scattered across the root model. The list
// lives in listview.go; the prompt in prompt.go; the chrome bars in titlebar.go
// and statusbar.go.
//
// Mutating methods use pointer receivers and return the pointer for chaining
// (e.g., m.args = m.args.EnterArg(t).WithSize(inner)). View is pure.
type ArgsView struct {
	st    styles
	inner int

	argTask   *Task
	argErr    string
	chips     []chip
	chipFocus int // -1 = input focused
}

// newArgsView builds an empty ArgsView with the given styles. chipFocus starts
// at -1 (input focused). The root constructs this once and reuses it across
// arg-mode entries via EnterArg/ExitArg.
func newArgsView(st styles) *ArgsView {
	return &ArgsView{st: st, chipFocus: -1}
}

// EnterArg transitions into argument-entry for the given task: it builds the
// chip slice from t.Args (each option -> a chip{arg,label,value}; booleans with
// no options -> one chip with label=token), resets chipFocus to -1 (input
// focused), clears any prior argErr, and records argTask. The prompt reset and
// placeholder/context swap are the root's responsibility (it owns the Prompt).
// Returns the pointer for chaining.
func (a *ArgsView) EnterArg(t Task) *ArgsView {
	a.argTask = &t
	a.argErr = ""
	a.chipFocus = -1
	a.chips = nil
	for _, arg := range t.Args {
		for _, opt := range arg.Options {
			label := opt
			if arg.Boolean {
				label = arg.Token
			}
			a.chips = append(a.chips, chip{arg: arg, label: label, value: opt})
		}
		if arg.Boolean && len(arg.Options) == 0 {
			a.chips = append(a.chips, chip{arg: arg, label: arg.Token, value: arg.Token})
		}
	}
	return a
}

// ExitArg clears all argument-entry state (argTask, argErr, chips, chipFocus),
// leaving the view inert. The prompt reset and context swap back to list mode
// are the root's responsibility. Returns the pointer for chaining.
func (a *ArgsView) ExitArg() *ArgsView {
	a.argTask = nil
	a.argErr = ""
	a.chipFocus = -1
	a.chips = nil
	return a
}

// CycleChip advances (forward) or retreats (backward) the chip focus across the
// chips, wrapping between -1 (input focused) and the last chip. If there are no
// chips, this is a no-op. Returns the pointer for chaining.
func (a *ArgsView) CycleChip(forward bool) *ArgsView {
	if len(a.chips) == 0 {
		return a
	}
	if forward {
		a.chipFocus = (a.chipFocus + 1) % (len(a.chips) + 1)
		if a.chipFocus == len(a.chips) {
			a.chipFocus = -1
		}
	} else {
		if a.chipFocus < 0 {
			a.chipFocus = len(a.chips) - 1
		} else {
			a.chipFocus--
		}
	}
	return a
}

// ClearChipFocus resets chipFocus to -1 (input focused). Called by the root on
// esc while a chip is focused, or after appending a chip on enter. Returns the
// pointer for chaining.
func (a *ArgsView) ClearChipFocus() *ArgsView {
	a.chipFocus = -1
	return a
}

// DismissErr clears the argument error (called when the user resumes typing
// after a submission failure). Returns the pointer for chaining.
func (a *ArgsView) DismissErr() *ArgsView {
	a.argErr = ""
	return a
}

// SetErr records an argument-entry error (e.g., from a failed submitArgs).
// Returns the pointer for chaining.
func (a *ArgsView) SetErr(err string) *ArgsView {
	a.argErr = err
	return a
}

// FocusedChip returns the currently focused chip and ok=true when a chip is
// focused (chipFocus >= 0). The root calls this on enter-in-args to append the
// chip's token to the Prompt, then calls ClearChipFocus.
func (a *ArgsView) FocusedChip() (chip, bool) {
	if a.chipFocus < 0 || a.chipFocus >= len(a.chips) {
		return chip{}, false
	}
	return a.chips[a.chipFocus], true
}

// ChipFocus reports the current chip focus index, or -1 when the input is
// focused.
func (a *ArgsView) ChipFocus() int {
	return a.chipFocus
}

// Task returns the task under argument entry, or nil when not in arg mode. The
// root calls this to drive completeInvocation on submit.
func (a *ArgsView) Task() *Task {
	return a.argTask
}

// HasChips reports whether there are any option chips to cycle. The root uses
// this to decide whether tab cycles chips or is a no-op.
func (a *ArgsView) HasChips() bool {
	return len(a.chips) > 0
}

// WithSize returns the view with the panel's inner width, set on
// WindowSizeMsg. Returns the pointer for chaining.
func (a *ArgsView) WithSize(inner int) *ArgsView {
	a.inner = inner
	return a
}

// Submit assembles the final shell command from the stored task and the
// user-supplied argument line. It validates that required arguments are present
// and returns the assembled command string. This keeps command assembly behind
// the arg-entry seam instead of leaking it into the root model.
func (a *ArgsView) Submit(promptValue string) (string, error) {
	if a.argTask == nil {
		return "", fmt.Errorf("no task in argument-entry mode")
	}
	argumentLine := strings.TrimSpace(promptValue)
	if argumentLine == "" {
		for _, arg := range a.argTask.Args {
			if arg.Required {
				return "", fmt.Errorf(
					"%s: missing required argument %s",
					a.argTask.Name,
					arg.Token,
				)
			}
		}
	}
	return assembleInvocation(*a.argTask, argumentLine), nil
}

// View renders the complete argument-entry panel body: the framed arg list
// (rounded top cap, per-arg rows with option chips, rounded bottom cap) and
// the optional error line. It is pure — no state is mutated. frame is the
// panel border decorator (owned by the root); bodyHeight is the available row
// count for the framed interior (the root computes listHeight()-2).
//
// The live invocation preview is no longer rendered in-panel; the root's
// last-row overlay (renderScriptPreviewRow in view.go) is the single source
// of truth for the assembled command, so this method owns only the frame and
// the error line. The chrome layers (title bar, prompt row, status footer)
// are assembled by viewArgs in view_args.go.
func (a *ArgsView) View(frame Frame, bodyHeight int) string {
	st := a.st
	t := a.argTask

	var body []string
	body = append(body, frame.Blank())
	body = append(body, frame.Paint(
		st.sp.PaddingLeft(padX).Render("")+st.sMuted.Render(ui.LetterSpace("arguments")),
		st.sp,
	))
	body = append(body, frame.Blank())

	tokenW := 4
	for _, arg := range t.Args {
		tokenW = max(tokenW, lipgloss.Width(arg.Token))
	}

	chipIdx := 0
	for _, arg := range t.Args {
		tag := "OPTIONAL"
		tagStyle := st.sDim
		switch {
		case arg.Required:
			tag, tagStyle = "REQUIRED", st.sErr
		case arg.Boolean:
			tag = "FLAG"
		}
		row := st.sp.PaddingLeft(padX).Render("") +
			st.sAccent.Bold(true).Width(tokenW).Render(arg.Token) + st.sp.Render("  ") +
			tagStyle.Width(8).Render(tag) + st.sp.Render("  ") +
			st.sMuted.Render(arg.Description)
		body = append(body, frame.Paint(row, st.sp))

		nChips := len(arg.Options)
		if arg.Boolean && nChips == 0 {
			nChips = 1
		}
		if nChips > 0 {
			var chips []string
			for i := 0; i < nChips; i++ {
				c := a.chips[chipIdx]
				label := " " + c.label + " "
				if chipIdx == a.chipFocus {
					// Focused options use the phosphor selection treatment.
					chips = append(chips, st.selText.Bold(true).Render(label))
				} else {
					chips = append(chips, st.optChip.Render(label))
				}
				chipIdx++
			}
			row := st.sp.PaddingLeft(padX+tokenW+2).Render("") +
				strings.Join(chips, st.sp.Render(" "))
			body = append(body, frame.Paint(row, st.sp))
		}
		body = append(body, frame.Blank())
	}

	// Pad the framed body to a stable height.
	h := bodyHeight
	for len(body) < h {
		body = append(body, frame.Blank())
	}
	if len(body) > h {
		body = body[:h]
	}

	var errLine string
	if a.argErr != "" {
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(st.pal.Error))).
			Background(st.openColor)
		errLine = st.openSp.Width(a.inner + 2).MaxWidth(a.inner + 2).Render(
			st.openSp.PaddingLeft(padX).Render("") + errStyle.Render(a.argErr),
		)
	}

	parts := []string{frame.Top()}
	parts = append(parts, body...)
	parts = append(parts, frame.Bottom())
	if errLine != "" {
		parts = append(parts, errLine)
	}
	return strings.Join(parts, "\n")
}
