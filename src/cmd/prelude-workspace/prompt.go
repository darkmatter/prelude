package main

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
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
` + bashSelectionRC + bashCompletionRC

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
          $BASH_COMMAND != __prelude_ghostty_load_command && $BASH_COMMAND != __prelude_ghostty_restore_input &&
          $BASH_COMMAND != __prelude_ghostty_completion_* ]]; then
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
	end      int // exclusive byte offset in this output chunk
	kind     byte
	status   int
	sequence int
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
				if (marker.kind == 'Q' || marker.kind == 'R') && len(parts) >= 3 {
					marker.sequence, _ = strconv.Atoi(parts[2])
				}
				// L/Q/R are private readline load/query/apply acknowledgements.
				if strings.ContainsRune("ABCDLQR", rune(marker.kind)) {
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

// Shell markers own the lifecycle and input boundary. Retain only the prompt
// geometry needed to place completion, never a mirrored editable buffer.
type promptState struct {
	phase string
	ready bool

	anchor         int // editable-input position at OSC133 B, before echoed input
	anchorCols     int
	fingerprint    []promptCell
	wholeTail      bool
	promptRows     int // persistent display height, not retained PS1 source
	promptRowsCols int // measured width; zero means the layout is unknown

	promptBytes       []byte // temporary A..B capture
	promptStartColumn int
	promptStartCols   int
	promptCollecting  bool
	promptOverflow    bool
}

// Only display geometry and the rendered prompt tail survive B, never another
// editable buffer. Styles are irrelevant to relocation; cells preserve grapheme
// widths, including wide continuations. Huge/sparse prompts fail closed.
const (
	promptFingerprintCells = 256
	promptFingerprintBytes = 4096
)

type promptCell struct {
	content string
	width   int
}

// Called before each PTY slice reaches Ghostty. mark(A) starts the capture;
// mark(B) measures it against that exact stream boundary and discards the bytes.
func (p *promptState) promptOutput(data []byte) {
	if !p.promptCollecting || p.promptOverflow || len(data) == 0 {
		return
	}
	if len(data) > promptFingerprintBytes-len(p.promptBytes) {
		p.promptBytes, p.promptOverflow = nil, true
		return
	}
	if p.promptBytes == nil {
		p.promptBytes = make([]byte, 0, promptFingerprintBytes)
	}
	p.promptBytes = append(p.promptBytes, data...)
}

// This measures only linear output, not a second terminal model. Anything that
// could reposition/erase cells or render non-text fails closed to the viewport.
func (p *promptState) renderedPromptRows(frame terminalFrame) (rows, measuredCols int) {
	if !p.promptCollecting || p.promptOverflow || frame.AltScreen || frame.Cols < 1 || frame.Rows < 1 ||
		p.promptStartCols != frame.Cols || p.promptStartColumn < 0 ||
		p.promptStartColumn >= frame.Cols-1 || !utf8.Valid(p.promptBytes) {
		return frame.Rows, 0
	}
	data := string(p.promptBytes)
	for remaining := data; len(remaining) > 0; {
		sequence, _, n, state := ansi.DecodeSequence(remaining, 0, nil)
		if n < 1 || state != ansi.NormalState {
			return frame.Rows, 0
		}
		remaining = remaining[n:]
		if sequence[0] != '\x1b' {
			continue
		}
		switch {
		case strings.HasPrefix(sequence, "\x1b[") && strings.HasSuffix(sequence, "m") &&
			strings.Trim(sequence[2:len(sequence)-1], "0123456789;:") == "":
			// SGR changes styling, never geometry.
		case strings.HasPrefix(sequence, "\x1b]"):
			body := strings.TrimPrefix(sequence, "\x1b]")
			if strings.HasSuffix(body, "\a") {
				body = strings.TrimSuffix(body, "\a")
			} else if strings.HasSuffix(body, "\x1b\\") {
				body = strings.TrimSuffix(body, "\x1b\\")
			} else {
				return frame.Rows, 0
			}
			command, _, _ := strings.Cut(body, ";")
			code, err := strconv.Atoi(command)
			if err != nil {
				return frame.Rows, 0
			}
			switch code {
			case 0, 1, 2, 7, 8, 133: // titles, cwd, hyperlinks, shell markers
			default:
				return frame.Rows, 0 // other OSCs can render images or change layout
			}
		default:
			return frame.Rows, 0
		}
	}

	rows, column := 1, p.promptStartColumn
	// Strip first so styling/hyperlinks cannot split a combining/emoji cluster.
	for text := ansi.Strip(data); len(text) > 0; {
		cluster, width := ansi.FirstGraphemeCluster(text, ansi.GraphemeWidth)
		if len(cluster) == 0 {
			return frame.Rows, 0
		}
		text = text[len(cluster):]
		switch cluster {
		case "\r":
			column = 0
		case "\r\n":
			rows, column = rows+1, 0
		case "\n":
			rows++
			column = min(column, frame.Cols-1)
		case "\x00", "\a":
		default:
			if strings.ContainsFunc(cluster, unicode.IsControl) || width < 1 || width > frame.Cols {
				return frame.Rows, 0
			}
			// A filled last cell wraps only on the next printable cluster. A
			// wide cluster that cannot fit wraps before it is drawn, not within.
			if column+width > frame.Cols {
				rows, column = rows+1, 0
			}
			column += width
		}
	}
	if frame.CursorX != min(column, frame.Cols-1) {
		return frame.Rows, 0 // display widths/modes did not match the native frame
	}
	return rows, frame.Cols
}

func (p *promptState) capture(frame terminalFrame) {
	p.anchor = frame.CursorY*frame.Cols + frame.CursorX
	p.anchorCols = frame.Cols
	p.fingerprint, p.wholeTail = nil, false
	// The adapter does not expose pending-wrap: a cursor at the last column
	// cannot distinguish B before that cell from B after filling the row.
	if frame.AltScreen || frame.Cols < 1 || frame.CursorX == frame.Cols-1 || p.anchor < 0 || p.anchor > len(frame.Cells) {
		p.anchor = -1
		return
	}
	start := p.anchor - frame.CursorX
	bounded := max(start, p.anchor-promptFingerprintCells)
	bytes := 0
	for i := p.anchor - 1; i >= bounded; i-- {
		bytes += len(frame.Cells[i].Content)
		if bytes > promptFingerprintBytes {
			bounded = i + 1
			break
		}
	}
	// Do not begin a fingerprint in the continuation of a wide grapheme.
	for bounded < p.anchor && frame.Cells[bounded].Width == 0 {
		bounded++
	}
	for _, cell := range frame.Cells[bounded:p.anchor] {
		p.fingerprint = append(p.fingerprint, promptCell{cell.Content, cell.Width})
	}
	p.wholeTail = bounded == start
}

func (p *promptState) mark(marker promptMarker, frame terminalFrame) {
	switch marker.kind {
	case 'A':
		p.phase, p.ready, p.anchor = "prompt", false, -1
		p.promptBytes, p.promptCollecting, p.promptOverflow = nil, true, frame.AltScreen
		p.promptStartColumn, p.promptStartCols = frame.CursorX, frame.Cols
	case 'B':
		// Readline can redisplay only the final PS1 line, repeating B without
		// A. Keep full-prompt geometry only within the same lifecycle/width;
		// an unknown layout must continue using the current viewport height.
		if p.promptCollecting {
			p.promptRows, p.promptRowsCols = p.renderedPromptRows(frame)
		} else if !p.ready || frame.AltScreen || p.promptRowsCols < 1 || p.promptRowsCols != frame.Cols {
			p.promptRows, p.promptRowsCols = frame.Rows, 0
		}
		// This exact stream boundary, not the prompt's text/byte length, is
		// authoritative. The fingerprint only relocates it after a redraw.
		p.phase, p.ready = "prompt", true
		p.capture(frame)
		p.observe(frame)
	case 'C':
		p.phase, p.ready = "running", false
	case 'D':
		p.phase, p.ready = "prompt", false

	}
	if marker.kind == 'B' || marker.kind == 'C' || marker.kind == 'D' {
		p.promptBytes, p.promptCollecting, p.promptOverflow = nil, false, false
		p.promptStartColumn, p.promptStartCols = 0, 0
	}
}

// Relocate the semantic input boundary after native scroll/reflow/redraw. The
// adapter has no scrollback/wrap metadata, so an off-screen or clipped prompt
// tail is unknown and completion must wait for a readable redraw.
func (p *promptState) observe(frame terminalFrame) {
	if !p.ready {
		return
	}
	if frame.AltScreen || frame.Cols < 1 || len(frame.Cells) == 0 {
		p.anchor = -1
		return
	}
	cursor := frame.CursorY*frame.Cols + frame.CursorX
	anchor := p.anchor
	if p.anchorCols != frame.Cols || anchor > cursor || !p.promptAt(frame, anchor) {
		anchor = -1
		// Prefer a still-valid semantic anchor over identical text in input.
		// Only a complete tail starting at column zero can be relocated with
		// this viewport-only adapter. Select the latest match before cursor.
		if p.wholeTail && len(p.fingerprint) > 0 {
			for i := 0; i+len(p.fingerprint) <= cursor; i += frame.Cols {
				candidate := i + len(p.fingerprint)
				if p.promptAt(frame, candidate) {
					anchor = candidate
				}
			}
		}
	}
	p.anchor, p.anchorCols = anchor, frame.Cols
}

func (p *promptState) promptAt(frame terminalFrame, input int) bool {
	start := input - len(p.fingerprint)
	if len(p.fingerprint) == 0 || start < 0 || input > len(frame.Cells) {
		return false
	}
	for i, want := range p.fingerprint {
		cell := frame.Cells[start+i]
		if cell.Width != want.width || cell.Content != want.content {
			return false
		}
	}
	return true
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
