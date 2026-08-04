// PROTOTYPE — shell host with pinned panel + bottom status.
//
// Why pin was blank: child CUP/ED are absolute on the host. Pin owns rows 1..N
// but the shell still does ESC[1;1H / ESC[2J into those rows. Fix: remap child
// CSI into the shell strip; paint pin as plain absolute lines only.
//
//	cd src && go run ./cmd/prelude-shell-spike -shell-rows 10
//	cd src && go run ./cmd/prelude-shell-spike -shell-rows 1
//
// Ctrl+G cycles pin: off → motd → menu → docs → off
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const keyCycle = 0x07 // Ctrl+G
const statusDebounce = 8 * time.Millisecond

func main() {
	shellRows := flag.Int("shell-rows", 10, "shell I/O rows when pin is active")
	flag.Parse()
	if err := run(*shellRows); err != nil {
		fmt.Fprintln(os.Stderr, "prelude-shell-spike:", err)
		os.Exit(1)
	}
}

func run(shellRows int) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("needs a real TTY")
	}
	if shellRows < 1 {
		return fmt.Errorf("-shell-rows must be >= 1")
	}

	cols, rows, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}
	if rows < 4 {
		return fmt.Errorf("need ≥4 rows, got %d", rows)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	starshipPath, starshipCleanup, err := compactStarshipConfig()
	if err != nil {
		return fmt.Errorf("compact starship: %w", err)
	}
	defer starshipCleanup()

	env := append(os.Environ(),
		"PRELUDE_SHELL_SPIKE=1",
		"TERM="+envOr("TERM", "xterm-256color"),
	)
	if starshipPath != "" {
		env = replaceEnv(env, "STARSHIP_CONFIG", starshipPath)
	}

	geom := computeLayout(cols, rows, shellRows, pinOff)

	cmd := exec.Command(shell)
	cmd.Env = env
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(geom.shellH), Cols: uint16(geom.cols)})
	if err != nil {
		return fmt.Errorf("pty: %w", err)
	}
	defer func() { _ = ptmx.Close() }()
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(geom.shellH), Cols: uint16(geom.cols)})

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), oldState) }()

	var (
		mu       sync.Mutex
		coal     escCoalescer
		paintDue bool
		lastSafe time.Time
		pinBody  string
	)

	applyHostGeometry := func(g layoutGeom) {
		// DECSTBM for shell strip only. Pin/status live outside and must be
		// painted with margins temporarily cleared (see paintChrome).
		_, _ = fmt.Fprintf(os.Stdout, "\033[?6l\033[%d;%dr", g.shellTop, g.shellTop+g.shellH-1)
		_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(g.shellH), Cols: uint16(g.cols)})
	}

	// paintChrome: portable host overlay.
	//
	// DECSTBM alone is not enough — many terminals clamp CUP to the scroll
	// region (or honor DECOM origin mode). So for every chrome paint we:
	//   1. save cursor
	//   2. force absolute origin + full-screen margins
	//   3. draw pin (rows 1..pinH) + status (last row)
	//   4. restore shell scroll region
	//   5. restore cursor
	// Without step 2, the top pin stays blank while the bottom status
	// sometimes still "works" (clamped into the shell strip).
	paintChrome := func(g layoutGeom, body string) {
		var b strings.Builder
		b.WriteString("\0337")    // DECSC
		b.WriteString("\033[?6l") // DECOM off — CUP is screen-absolute
		b.WriteString("\033[r")   // full-screen scroll region

		if g.pinH > 0 {
			lines := strings.Split(body, "\n")
			for i := 0; i < g.pinH; i++ {
				row := i + 1
				if i == 0 {
					// Reverse-video title — impossible to miss if paint lands.
					fmt.Fprintf(&b, "\033[%d;1H\033[2K\033[0;7m", row)
				} else {
					fmt.Fprintf(&b, "\033[%d;1H\033[2K\033[0m", row)
				}
				if i < len(lines) {
					b.WriteString(lines[i])
				}
				b.WriteString("\033[0m")
			}
		}

		fmt.Fprintf(&b, "\033[%d;1H\033[2K\033[0;90m%s\033[0m", g.statusRow, g.statusLine())

		// Re-apply shell strip margins, then put the cursor back.
		fmt.Fprintf(&b, "\033[%d;%dr", g.shellTop, g.shellTop+g.shellH-1)
		b.WriteString("\0338") // DECRC

		_, _ = os.Stdout.WriteString(b.String())
		_ = os.Stdout.Sync()
	}

	initialPaint := func(g layoutGeom) {
		_, _ = fmt.Fprintf(os.Stdout, "\033[?6l\033[r\033[1;1H\033[2J\033[H")
		applyHostGeometry(g)
		shellBottom := g.shellTop + g.shellH - 1
		_, _ = fmt.Fprintf(os.Stdout, "\033[%d;1H", shellBottom)
		paintChrome(g, "")
	}

	reflow := func(g layoutGeom, body string) {
		applyHostGeometry(g)
		paintChrome(g, body)
	}

	cyclePin := func() {
		// Show loading chrome immediately, then capture (may take ~1–2s).
		mu.Lock()
		geom.pin = geom.pin.next()
		geom = computeLayout(geom.cols, geom.totalH, shellRows, geom.pin)
		if geom.pin != pinOff {
			pinBody = pinChrome(geom.cols, geom.pinH, geom.pin.label(), []string{"loading…"}, "")
			reflow(geom, pinBody)
		} else {
			pinBody = ""
			reflow(geom, "")
		}
		pin := geom.pin
		cols, pinH := geom.cols, geom.pinH
		mu.Unlock()

		var body string
		if pin != pinOff {
			body = refreshPinContent(pin, cols, pinH, env)
		}

		mu.Lock()
		// Ignore stale capture if user cycled again while we were loading.
		if geom.pin == pin {
			pinBody = body
			reflow(geom, pinBody)
			_, _ = ptmx.Write([]byte{0x0c})
		}
		lastSafe = time.Now()
		paintDue = false
		mu.Unlock()
	}

	writeShell := func(p []byte) {
		mu.Lock()
		defer mu.Unlock()

		out, safe := coal.Feed(p)
		if len(out) > 0 {
			if geom.pinH > 0 {
				out = remapShellOutput(out, geom.shellTop, geom.shellH, geom.cols)
			}
			if len(out) > 0 {
				_, _ = os.Stdout.Write(out)
			}
		}
		if !safe {
			paintDue = true
			return
		}
		// When pin is on, always re-stamp chrome after shell output (remap is
		// not perfect; pin paint is cheap plain text).
		if geom.pinH > 0 {
			paintChrome(geom, pinBody)
			paintDue = false
			lastSafe = time.Now()
			return
		}
		if paintDue || time.Since(lastSafe) >= statusDebounce {
			paintChrome(geom, "")
			paintDue = false
			lastSafe = time.Now()
		}
	}

	mu.Lock()
	initialPaint(geom)
	lastSafe = time.Now()
	mu.Unlock()

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			c, r, err := term.GetSize(int(os.Stdout.Fd()))
			if err != nil || r < 4 {
				continue
			}
			mu.Lock()
			pin := geom.pin
			geom = computeLayout(c, r, shellRows, pin)
			mu.Unlock()

			var body string
			if pin != pinOff {
				body = refreshPinContent(pin, c, geom.pinH, env)
			}

			mu.Lock()
			if geom.pin == pin {
				pinBody = body
				reflow(geom, pinBody)
			}
			lastSafe = time.Now()
			mu.Unlock()
		}
	}()

	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				var forward []byte
				for i := 0; i < n; i++ {
					if buf[i] == keyCycle {
						if len(forward) > 0 {
							_, _ = ptmx.Write(forward)
							forward = forward[:0]
						}
						cyclePin()
						continue
					}
					forward = append(forward, buf[i])
				}
				if len(forward) > 0 {
					_, _ = ptmx.Write(forward)
				}
			}
			if err != nil {
				return
			}
		}
	}()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := ptmx.Read(buf)
		if n > 0 {
			writeShell(buf[:n])
		}
		if readErr != nil {
			mu.Lock()
			if tail := coal.Flush(); len(tail) > 0 {
				if geom.pinH > 0 {
					tail = remapShellOutput(tail, geom.shellTop, geom.shellH, geom.cols)
				}
				_, _ = os.Stdout.Write(tail)
			}
			mu.Unlock()
			break
		}
	}

	_ = cmd.Wait()
	mu.Lock()
	_, _ = fmt.Fprintf(os.Stdout, "\033[r\033[1;1H\033[2J\033[H")
	mu.Unlock()
	return nil
}

func fitStatus(s string, cols int) string {
	return padTruncate(s, cols)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	found := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			out = append(out, prefix+value)
			found = true
			continue
		}
		out = append(out, e)
	}
	if !found {
		out = append(out, prefix+value)
	}
	return out
}
