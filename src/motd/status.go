package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// miniDotFrames mirrors charm.land/bubbles/v2/spinner.MiniDot (kept local so
// the vendored motd binary does not need the full bubbles spinner package).
var miniDotFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// resolveHeaderStatuses runs live checks for header badges, optionally showing
// a MiniDot spinner on stderr while each check runs. Static badges (no check)
// are left as-is with Level "static".
func resolveHeaderStatuses(cfg *Config, rt Runtime) []string {
	items := cfg.Header.Status
	if len(items) == 0 {
		return nil
	}
	var diagnostics []string

	interactive := term.IsTerminal(int(os.Stderr.Fd()))
	// Resolve sequentially so the spinner line stays readable; checks are
	// usually cheap shell probes.
	for i := range items {
		item := &items[i]
		if item.Check == "" {
			if item.Status != "" || item.Label != "" {
				item.Level = "static"
			}
			continue
		}

		label := item.Label
		if label == "" {
			label = "check"
		}

		var stop func()
		if interactive {
			stop = spinStatus(os.Stderr, label, cfg.ColorProfile)
		}
		ok, out := rt.Check(item.Check)
		if stop != nil {
			stop()
		}

		if ok {
			item.Level = "success"
			switch item.Output {
			case "light":
				item.Status = ""
			default:
				switch {
				case item.Ok != "":
					item.Status = item.Ok
					if out != "" {
						diagnostics = append(diagnostics, out)
					}
				case out != "":
					item.Status = firstLine([]byte(out))
				default:
					item.Status = "ok"
				}
			}
		} else {
			item.Level = "error"
			if item.FailLevel == "warning" {
				item.Level = "warning"
			}
			switch item.Output {
			case "light":
				item.Status = ""
			default:
				switch {
				case item.Fail != "":
					item.Status = item.Fail
					if out != "" {
						diagnostics = append(diagnostics, out)
					}
				case out != "":
					item.Status = firstLine([]byte(out))
				default:
					item.Status = "fail"
				}
			}
		}
	}
	cfg.Header.Status = items
	return diagnostics
}

// spinStatus writes an animated MiniDot spinner for label to w until the
// returned stop func is called. stop clears the line.
func spinStatus(w io.Writer, label, colorProfile string) func() {
	fps := time.Second / 12
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	_ = colorProfile

	var (
		mu      sync.Mutex
		stopped bool
		done    = make(chan struct{})
	)

	go func() {
		i := 0
		t := time.NewTicker(fps)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				mu.Lock()
				if stopped {
					mu.Unlock()
					return
				}
				frame := miniDotFrames[i%len(miniDotFrames)]
				i++
				// \r rewrite single status line on stderr.
				fmt.Fprintf(w, "\r%s %s", accent.Render(frame), dim.Render(label))
				mu.Unlock()
			}
		}
	}()

	return func() {
		mu.Lock()
		stopped = true
		mu.Unlock()
		close(done)
		// Clear the spinner line.
		fmt.Fprintf(w, "\r%s\r", strings.Repeat(" ", len(label)+4))
	}
}
