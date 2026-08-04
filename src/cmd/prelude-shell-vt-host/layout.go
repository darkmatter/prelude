package main

// Frame geometry. Everything here is pure arithmetic so the interesting
// question — "which rows belong to whom" — can be tested without a terminal.

// band is a horizontal slice of the frame covering rows [top, top+height).
// Zero height means the band is not present in this layout.
type band struct {
	top    int
	height int
}

func (b band) bottom() int { return b.top + b.height }

func (b band) contains(row int) bool { return row >= b.top && row < b.bottom() }

// pinMode selects which pinned surface occupies the band above the shell.
type pinMode int

const (
	pinOff pinMode = iota
	pinMotd
	pinMenu
	pinDocs
)

// pinLabels doubles as the cycle order: ^G advances one step and wraps.
var pinLabels = [...]string{"off", "motd", "menu", "docs"}

func (m pinMode) next() pinMode { return (m + 1) % pinMode(len(pinLabels)) }

func (m pinMode) on() bool { return m != pinOff }

// live reports whether this mode is backed by a child that keeps running in
// the band, rather than a command photographed once and killed.
//
// Docs is the whole reason the distinction exists: it is an interactive TUI,
// so a snapshot of its first frame is a picture of a program nobody can use.
// A live mode is driven by keystrokes and therefore needs focus, resizing,
// and a shutdown of its own.
func (m pinMode) live() bool { return m == pinDocs }

func (m pinMode) label() string {
	if m < 0 || int(m) >= len(pinLabels) {
		return "?"
	}
	return pinLabels[m]
}

const (
	// minFrameRows is one pin row, one shell row, and the status row. Below
	// that there is nothing coherent left to lay out.
	minFrameRows = 3
	minFrameCols = 1
)

type layout struct {
	cols int
	rows int
	// pin covers the pinned header row plus whatever panel body fits under
	// it. Height is zero when the pin is off.
	pin       band
	shell     band
	statusRow int
}

// panelBody is the part of the pin band a captured panel may draw into: the
// pin band minus its header row. Height can be zero on very short terminals,
// which is the signal not to spawn a capture at all.
func (l layout) panelBody() band {
	if l.pin.height <= 1 {
		return band{top: l.pin.bottom()}
	}
	return band{top: l.pin.top + 1, height: l.pin.height - 1}
}

// computeLayout divides a frame between the pinned surface, the child shell,
// and the status row. wantShellRows is the shell height requested by the
// operator; it is honoured only as far as the frame allows, and the shell
// always keeps at least one row.
func computeLayout(cols, rows, wantShellRows int, pin pinMode) layout {
	cols = max(cols, minFrameCols)
	rows = max(rows, minFrameRows)
	wantShellRows = max(wantShellRows, 1)

	result := layout{cols: cols, rows: rows, statusRow: rows - 1}
	body := rows - 1 // everything above the status row

	if !pin.on() {
		result.shell = band{height: body}
		return result
	}

	// Reserve a row for the pin header, and prefer to leave a second one so a
	// captured panel has somewhere to land. On a frame too short for both the
	// shell still wins: a header-only pin is legible, a zero-row shell is not.
	shellRows := min(wantShellRows, max(body-2, 1))
	result.pin = band{height: body - shellRows}
	result.shell = band{top: result.pin.height, height: shellRows}
	return result
}

func clamp(value, low, high int) int { return min(max(value, low), high) }
