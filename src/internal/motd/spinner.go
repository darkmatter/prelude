package motd

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"charm.land/lipgloss/v2"
)

// miniDotFrames mirrors charm.land/bubbles/v2/spinner.MiniDot (kept local so
// the vendored motd binary does not need the full bubbles spinner package).
var miniDotFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is a React-style, one-component-per-file component that renders an
// animated MiniDot spinner for a status label on an output writer until the
// returned stop func is called. It is standalone — it takes all inputs as
// params and holds no renderer context, since it runs during status resolution
// (before the renderer exists).
type Spinner struct{}

// Render writes an animated MiniDot spinner for label to w until the returned
// stop func is called. stop clears the line.
func (s Spinner) Render(w io.Writer, label string) func() {
	fps := time.Second / 12
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

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
