package main

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"
)

// refreshPinContent builds a fixed-height pin panel with TTY colors preserved.
// Layout CSI (cursor/erase/modes) is stripped so paint stays row-absolute;
// SGR (color/style) is kept.
func refreshPinContent(mode pinMode, cols, rows int, env []string) string {
	if mode == pinOff || rows <= 0 {
		return ""
	}

	var raw string
	var src string
	switch mode {
	case pinMotd:
		src = "motd"
		if out, err := captureOnPTY("motd", nil, cols, max(rows*2, 24), env, 2500*time.Millisecond); err == nil {
			raw = out
		}
	case pinMenu:
		src = "x --list"
		if out, err := captureOnPTY("x", []string{"--list"}, cols, max(rows*2, 24), env, 2500*time.Millisecond); err == nil {
			raw = out
		} else if out, err := captureOnPTY("menu", []string{"-x", "--list"}, cols, max(rows*2, 24), env, 2500*time.Millisecond); err == nil {
			src = "menu -x --list"
			raw = out
		}
	case pinDocs:
		return pinChrome(cols, rows, "docs", []string{
			"Interactive docs needs a real VT embed later.",
			"Placeholder only — type `docs` in the shell strip for the TUI.",
		}, "")
	}

	colored := strings.TrimSpace(sanitizePinANSI(raw))
	var body []string
	for _, line := range strings.Split(colored, "\n") {
		// Keep blank visual lines out; measure without ANSI.
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			continue
		}
		body = append(body, strings.TrimRight(line, " \t"))
	}
	note := ""
	if len(body) == 0 {
		note = fmt.Sprintf("no output from %q (not on PATH?)", src)
		body = []string{
			"Pin capture returned empty text.",
			"Run from a direnv/nix-develop shell so motd/x are on PATH.",
			fmt.Sprintf("PATH head: %s", pathHead(env)),
		}
	}
	return pinChrome(cols, rows, mode.label(), body, note)
}

func pathHead(env []string) string {
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			p := strings.TrimPrefix(e, "PATH=")
			if len(p) > 60 {
				return p[:60] + "…"
			}
			return p
		}
	}
	return "(no PATH in env)"
}

// pinChrome: reverse-video title bar + body lines (may contain SGR colors).
func pinChrome(cols, rows int, title string, body []string, note string) string {
	if rows <= 0 {
		return ""
	}
	lines := make([]string, rows)
	titleLine := fmt.Sprintf("### PIN:%s ###  Ctrl-G=cycle  ###", strings.ToUpper(title))
	if note != "" {
		titleLine = fmt.Sprintf("### PIN:%s (%s) ###", strings.ToUpper(title), note)
	}
	lines[0] = padTruncate(titleLine, cols)

	bodyRows := rows - 1
	for i := 0; i < bodyRows; i++ {
		if i < len(body) {
			lines[i+1] = padTruncate(body[i], cols)
		} else {
			lines[i+1] = padTruncate("", cols)
		}
	}
	return strings.Join(lines, "\n")
}

// padTruncate sizes by visible cells, preserving ANSI SGR in the result.
func padTruncate(s string, cols int) string {
	if cols <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w > cols {
		// Truncate is ANSI-aware; paint always appends SGR reset after the line.
		return ansi.Truncate(s, cols, "…")
	}
	if w < cols {
		return s + strings.Repeat(" ", cols-w)
	}
	return s
}

// sanitizePinANSI keeps SGR color/style sequences (CSI … m) and printable text,
// drops cursor/erase/mode/OSC sequences that would fight host pin geometry.
func sanitizePinANSI(s string) string {
	var out strings.Builder
	b := []byte(s)
	i := 0
	for i < len(b) {
		if b[i] != 0x1b || i+1 >= len(b) {
			c := b[i]
			// Keep printable + tab/newline; drop CR and other C0.
			if c == '\n' || c == '\t' || c >= 0x20 {
				out.WriteByte(c)
			}
			i++
			continue
		}

		switch b[i+1] {
		case '[': // CSI
			j := i + 2
			for j < len(b) && !(b[j] >= 0x40 && b[j] <= 0x7e) {
				j++
			}
			if j >= len(b) {
				// incomplete — drop rest
				return out.String()
			}
			// Keep SGR only (colors, bold, etc.)
			if b[j] == 'm' {
				out.Write(b[i : j+1])
			}
			i = j + 1

		case ']': // OSC — drop (title, hyperlinks, …)
			j := i + 2
			for j < len(b) {
				if b[j] == 0x07 { // BEL
					j++
					break
				}
				if b[j] == 0x1b && j+1 < len(b) && b[j+1] == '\\' { // ST
					j += 2
					break
				}
				j++
			}
			i = j

		case 'P', 'X', '^', '_': // DCS/SOS/PM/APC — drop to ST
			j := i + 2
			for j < len(b) {
				if b[j] == 0x1b && j+1 < len(b) && b[j+1] == '\\' {
					j += 2
					break
				}
				if b[j] == 0x07 {
					j++
					break
				}
				j++
			}
			i = j

		case '(', ')', '*', '+', '-', '.', '/': // charset designate
			if i+2 < len(b) {
				i += 3
			} else {
				i = len(b)
			}

		default:
			// ESC 7/8, ESC c, etc. — drop two-byte sequence
			i += 2
		}
	}
	return out.String()
}

func captureOnPTY(name string, args []string, cols, rows int, env []string, timeout time.Duration) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(path, args...)
	cmd.Env = append([]string{}, env...)
	// Force the colored TTY path: strip pure/no-color, force CLICOLOR.
	cmd.Env = filterEnv(cmd.Env, "PRELUDE_MOTD_PURE")
	cmd.Env = filterEnv(cmd.Env, "NO_COLOR")
	cmd.Env = replaceEnv(cmd.Env, "CLICOLOR_FORCE", "1")
	cmd.Env = replaceEnv(cmd.Env, "CLICOLOR", "1")
	cmd.Env = replaceEnv(cmd.Env, "TERM", envOr("TERM", "xterm-256color"))
	if ct := osGetenvFrom(env, "COLORTERM"); ct != "" {
		cmd.Env = replaceEnv(cmd.Env, "COLORTERM", ct)
	} else {
		cmd.Env = replaceEnv(cmd.Env, "COLORTERM", "truecolor")
	}
	cmd.Env = replaceEnv(cmd.Env, "COLUMNS", fmt.Sprintf("%d", cols))
	cmd.Env = replaceEnv(cmd.Env, "LINES", fmt.Sprintf("%d", rows))

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: uint16(max(rows, 1)),
		Cols: uint16(max(cols, 1)),
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = ptmx.Close() }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, ptmx)
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		_ = cmd.Wait()
	case <-timer.C:
		_ = cmd.Process.Kill()
		<-done
		_ = cmd.Wait()
	}
	return buf.String(), nil
}

func osGetenvFrom(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

func filterEnv(env []string, key string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			continue
		}
		out = append(out, e)
	}
	return out
}
