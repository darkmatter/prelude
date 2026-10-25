package main

import (
	"strconv"
	"strings"
	"unicode"
)

// The fixed prompt remains the default; neither prompt mode uses it as an
// input anchor. OSC133 B supplies the editable-input boundary.
const workspacePrompt = "prelude $ "

// No user rc, inputrc, or persistent history. In particular, do not unset or
// assign readonly Bash environment variables such as SHELLOPTS or BASHOPTS.
// Disabling inherited errexit/nounset/functrace keeps the hooks self-contained.
//
// OSC133 lifecycle: A = prompt begins, B = editable input begins,
// C = command starts, D = command finishes with its exit status.
// Bash's \[...\] wrappers exclude these invisible bytes from readline's widths.
// Prompts restore a blinking bar after child apps change it; View must still
// honor those apps' cursor styles while they own the terminal.
const bashCommonRC = `set +e +u +T
set -o emacs
builtin shopt -s promptvars
__prelude_ghostty_primary_prompt=0
__prelude_ghostty_prompt_generation=0
builtin trap - DEBUG
PS0=''
PS1=''
# A secondary prompt is not a safe place to submit a queued menu command.
PS2='\[\e[5 q\e]133;A;$((__prelude_ghostty_primary_prompt=0))\a\]... '
PROMPT_COMMAND=''
HISTFILE=/dev/null
builtin bind 'set enable-bracketed-paste on'
builtin bind 'set show-all-if-ambiguous on'
# Entry output belongs to the main shell, before either prompt mode's hooks.
# A failed banner must not prevent the user from getting an interactive prompt.
if ! command motd; then
    builtin printf '%s\n' 'prelude-workspace: startup motd failed; continuing with the shell' >&2
fi
# Load only catalogue completion: full activation would also install ble.sh,
# change the prompt/status row, and print another MOTD.
if [[ -n ${PRELUDE_COMPLETION_INIT-} ]]; then
    if ! builtin source "$PRELUDE_COMPLETION_INIT"; then
        builtin printf '%s\n' 'prelude-workspace: completion init failed; continuing with the shell' >&2
    fi
fi
` + bashSelectionRC

// The fixed prompt's DEBUG guard emits C once per submitted command line, not
// once per simple command/pipeline stage. Starship uses its own PS0 instead.
const bashRC = bashCommonRC + `PS1='\[\e[5 q\e]133;A\a\]prelude $ \[\e]133;B\a\]'
__prelude_ghostty_at_prompt=0
__prelude_ghostty_seen_prompt=0
__prelude_ghostty_prompt() {
    local status=$?
    __prelude_ghostty_primary_prompt=1
    ((__prelude_ghostty_prompt_generation+=1))
    if [[ $__prelude_ghostty_seen_prompt == 1 ]]; then
        builtin printf '\e]133;D;%s\a' "$status"
    fi
    __prelude_ghostty_seen_prompt=1
    __prelude_ghostty_at_prompt=1
    return "$status"
}
__prelude_ghostty_preexec() {
    if [[ $__prelude_ghostty_at_prompt == 1 && $BASH_COMMAND != __prelude_ghostty_prompt &&
          $BASH_COMMAND != __prelude_ghostty_load_command && $BASH_COMMAND != __prelude_ghostty_restore_input ]]; then
        __prelude_ghostty_at_prompt=0
        __prelude_ghostty_primary_prompt=0
        builtin printf '\e]133;C\a'
    fi
}
PROMPT_COMMAND=__prelude_ghostty_prompt
builtin trap '__prelude_ghostty_preexec' DEBUG
`

// Appended to the installed `starship init bash --print-full-init` script.
// Its precmd must run FIRST: it captures both $? and PIPESTATUS before any of
// our metadata commands. On Bash >=4.4 its PS0 already owns the command timer;
// append C without replacing that expansion or installing a DEBUG trap.
const bashStarshipRC = `# Starship replaces PS2; retain its styling but mark continuation as not ready.
PS2='\[\e[5 q\e]133;A;$((__prelude_ghostty_primary_prompt=0))\a\]'"$PS2"
__prelude_ghostty_seen_prompt=0
__prelude_ghostty_starship_prompt() {
    starship_precmd
    __prelude_ghostty_primary_prompt=1
    ((__prelude_ghostty_prompt_generation+=1))
    if [[ $__prelude_ghostty_seen_prompt == 1 ]]; then
        builtin printf '\e]133;D;%s\a' "$STARSHIP_CMD_STATUS"
    fi
    __prelude_ghostty_seen_prompt=1
    PS1='\[\e[5 q\e]133;A\a\]'"$PS1"'\[\e]133;B\a\]'
    return "$STARSHIP_CMD_STATUS"
}
PROMPT_COMMAND=__prelude_ghostty_starship_prompt
PS0+=$'\e]133;C;''$((__prelude_ghostty_primary_prompt=0))'$'\a'
`

type promptMarker struct {
	end    int // exclusive byte offset in this output chunk
	kind   byte
	status int
}

// This is only a bounded OSC133 observer, not a terminal emulator. All bytes,
// including these hooks, still go through libghostty-vt. Ignore other control
// strings (notably DCS) so an embedded OSC does not masquerade as a shell hook.
type promptParser struct {
	state uint8
	body  [128]byte
	n     int
}

const (
	promptGround uint8 = iota
	promptEscape
	promptOSC
	promptOSCEscape
	promptString
	promptStringEscape
)

func (p *promptParser) scan(data []byte) []promptMarker {
	var markers []promptMarker
	finish := func(end int) {
		if p.n <= len(p.body) {
			parts := strings.Split(string(p.body[:p.n]), ";")
			if len(parts) >= 2 && parts[0] == "133" && len(parts[1]) == 1 {
				marker := promptMarker{end: end, kind: parts[1][0], status: -1}
				if marker.kind == 'D' && len(parts) >= 3 {
					if code, err := strconv.Atoi(parts[2]); err == nil && code >= 0 && code <= 255 {
						marker.status = code
					}
				}
				// L is a private readline load acknowledgement.
				if strings.ContainsRune("ABCDL", rune(marker.kind)) {
					markers = append(markers, marker)
				}
			}
		}
		p.state, p.n = promptGround, 0
	}
	for i, b := range data {
		switch p.state {
		case promptGround:
			if b == '\x1b' {
				p.state = promptEscape
			}
		case promptEscape:
			switch b {
			case ']':
				p.state, p.n = promptOSC, 0
			case 'P', 'X', '^', '_':
				p.state = promptString
			case '\x1b':
			default:
				p.state = promptGround
			}
		case promptOSC:
			switch b {
			case '\a':
				finish(i + 1)
			case '\x1b':
				p.state = promptOSCEscape
			default:
				if p.n < len(p.body) {
					p.body[p.n] = b
				}
				p.n = min(p.n+1, len(p.body)+1)
			}
		case promptOSCEscape:
			if b == '\\' {
				finish(i + 1)
			} else {
				p.n = len(p.body) + 1 // malformed OSC; discard until its terminator
				p.state = promptOSC
			}
		case promptString:
			if b == '\x1b' {
				p.state = promptStringEscape
			}
		case promptStringEscape:
			if b == '\\' {
				p.state = promptGround
			} else if b != '\x1b' {
				p.state = promptString
			}
		}
	}
	return markers
}

// Shell markers own the lifecycle. anchor is the editable-input position at
// OSC133 B on the live screen; it is a display boundary only, never a
// mirrored editable buffer. It is invalidated rather than relocated when
// geometry changes, and recaptured at the next B.
type promptState struct {
	phase string
	ready bool

	anchor         int // editable-input position at OSC133 B, before echoed input
	anchorCols     int
	pendingCapture bool // B arrived while scrolled; retry on the live screen
}

func (p *promptState) capture(frame terminalFrame) {
	p.pendingCapture = false
	p.anchor = frame.CursorY*frame.Cols + frame.CursorX
	p.anchorCols = frame.Cols
	if frame.Scrolled {
		// Cursor coordinates describe the live screen, not these history
		// cells. Retry on the first live snapshot within the same lifecycle.
		p.anchor, p.pendingCapture = -1, true
		return
	}
	// The adapter does not expose pending-wrap: a cursor at the last column
	// cannot distinguish B before that cell from B after filling the row.
	if frame.AltScreen || frame.Cols < 1 || frame.CursorX == frame.Cols-1 || p.anchor < 0 || p.anchor > len(frame.Cells) {
		p.anchor = -1
	}
}

func (p *promptState) mark(marker promptMarker, frame terminalFrame) {
	switch marker.kind {
	case 'A':
		p.phase, p.ready, p.anchor, p.pendingCapture = "prompt", false, -1, false
	case 'B':
		p.phase, p.ready = "prompt", true
		p.capture(frame)
	case 'C':
		p.phase, p.ready, p.pendingCapture = "running", false, false
	case 'D':
		p.phase, p.ready, p.pendingCapture = "prompt", false, false
	}
}

func (p *promptState) observe(frame terminalFrame) {
	if !p.ready {
		return
	}
	if frame.AltScreen || frame.Cols < 1 || len(frame.Cells) == 0 {
		p.anchor = -1
		return
	}
	if frame.Scrolled {
		// Only the viewport moved; the live prompt and its anchor are unchanged.
		return
	}
	if p.pendingCapture {
		p.capture(frame)
	}
	if p.anchorCols != frame.Cols || p.anchor > frame.CursorY*frame.Cols+frame.CursorX {
		p.anchor = -1
	}
	p.anchorCols = frame.Cols
}

// Command text and OSC titles are display data, not host escape sequences.
// Remove controls before they enter chrome or the outer window title.
func displayText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
}
