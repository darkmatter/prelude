package main

// escCoalescer rewrites a PTY byte stream so incomplete ANSI/VT escape
// sequences are never flushed mid-sequence. That lets us inject status-line
// paints only at safe boundaries.
//
// Artifacts like visible "135m" or ";46;56" happen when a CSI such as
// ESC[38;5;135m is split across Read() calls and we paint (ESC7 … ESC8)
// between the pieces — the terminal then treats the tail as plain text.
//
// This is NOT a full VT emulator (no cursor model). It only answers: "is it
// safe to inject host chrome right now?"

type escState int

const (
	stGround escState = iota
	stEsc             // saw ESC
	stCSI             // ESC [ ...
	stOSC             // ESC ] ... (or other string types ending in BEL/ST)
	stEscInt          // ESC + intermediate (charset, etc.) awaiting final
)

// string terminators: BEL, or ESC \ (ST)
const bel = 0x07

type escCoalescer struct {
	state  escState
	pending []byte
}

// Feed appends raw PTY bytes and returns complete ground text + finished
// escape sequences ready to write. Incomplete sequence tails stay buffered.
func (c *escCoalescer) Feed(p []byte) (out []byte, safe bool) {
	for _, b := range p {
		switch c.state {
		case stGround:
			if b == 0x1b {
				// Flush plain text before entering ESC.
				if len(c.pending) > 0 {
					out = append(out, c.pending...)
					c.pending = c.pending[:0]
				}
				c.pending = append(c.pending, b)
				c.state = stEsc
				continue
			}
			c.pending = append(c.pending, b)

		case stEsc:
			c.pending = append(c.pending, b)
			switch b {
			case '[':
				c.state = stCSI
			case ']', 'P', 'X', '^', '_':
				// OSC / DCS / SOS / PM / APC — string terminated by BEL or ST.
				c.state = stOSC
			case ' ', '#', '%', '(', ')', '*', '+':
				// Intermediate; need one more final byte.
				c.state = stEscInt
			default:
				// Two-byte ESC sequence (e.g. ESC 7, ESC c) — complete.
				out = append(out, c.pending...)
				c.pending = c.pending[:0]
				c.state = stGround
			}

		case stCSI:
			c.pending = append(c.pending, b)
			// CSI: parameter/intermediate bytes then final in @-~ (0x40-0x7e).
			if b >= 0x40 && b <= 0x7e {
				out = append(out, c.pending...)
				c.pending = c.pending[:0]
				c.state = stGround
			}

		case stOSC:
			c.pending = append(c.pending, b)
			// BEL ends OSC; ST is ESC \ — if we see ESC inside string, wait for \.
			if b == bel {
				out = append(out, c.pending...)
				c.pending = c.pending[:0]
				c.state = stGround
			} else if b == '\\' && len(c.pending) >= 2 && c.pending[len(c.pending)-2] == 0x1b {
				out = append(out, c.pending...)
				c.pending = c.pending[:0]
				c.state = stGround
			}

		case stEscInt:
			c.pending = append(c.pending, b)
			// Final byte completes charset / hash sequences.
			out = append(out, c.pending...)
			c.pending = c.pending[:0]
			c.state = stGround
		}
	}

	// Flush any ground-mode pending plain text so typing isn't delayed.
	// Escape tails stay in pending until complete.
	if c.state == stGround && len(c.pending) > 0 {
		out = append(out, c.pending...)
		c.pending = c.pending[:0]
	}

	// Safe to paint host chrome only when not mid-sequence.
	safe = c.state == stGround && len(c.pending) == 0
	return out, safe
}

// Flush forces out any buffered bytes (on PTY close). May include incomplete
// sequences — last resort so nothing is lost.
func (c *escCoalescer) Flush() []byte {
	if len(c.pending) == 0 {
		return nil
	}
	out := append([]byte(nil), c.pending...)
	c.pending = c.pending[:0]
	c.state = stGround
	return out
}
