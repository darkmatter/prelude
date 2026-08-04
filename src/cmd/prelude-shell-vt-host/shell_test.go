package main

import (
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
)

var blankPaint = paint{fill: uv.Cell{Content: " ", Width: 1}}

// testShell is a live child plus the exit signal its pump reports.
type testShell struct {
	*shell
	exited chan error
}

// startTestShell runs a bare, rc-free sh so the test never depends on the
// developer's dotfiles, and starts the pump so child output is absorbed.
//
// LANG matters: /bin/sh is bash, and readline in the C locale silently drops
// 8-bit input instead of inserting it, so wide graphemes never reach the
// child. That is the child's locale handling, not the host's forwarding, which
// is byte-exact — but without this the wide-grapheme tests would measure the
// wrong thing.
func startTestShell(t *testing.T, cols, rows int) *testShell {
	t.Helper()
	child, err := startShell(shellSpec{
		command:    "/bin/sh",
		cols:       cols,
		rows:       rows,
		scrollback: 200,
		env: []string{
			"PS1=$ ",
			"TERM=xterm-256color",
			"LANG=en_US.UTF-8",
			"HOME=" + t.TempDir(),
		},
	})
	if err != nil {
		t.Fatalf("start shell: %v", err)
	}
	t.Cleanup(child.stop)

	live := &testShell{shell: child, exited: make(chan error, 1)}
	child.pump(func() {}, func(exitErr error) { live.exited <- exitErr })
	return live
}

// run types a command and presses Enter, exactly as the host forwards input.
func (s *testShell) run(command string) {
	s.sendText(command)
	s.sendKey(uv.KeyPressEvent{Code: uv.KeyEnter})
}

// await polls the child's screen until check passes. The child is a real
// process, so every assertion about its output is inherently asynchronous.
func (s *testShell) await(t *testing.T, what string, check func(string) bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var latest string
	for time.Now().Before(deadline) {
		latest = s.text()
		if check(latest) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s; last screen:\n%s", what, latest)
}

func TestChildOutputReachesTheVirtualScreen(t *testing.T) {
	child := startTestShell(t, 40, 6)

	child.run("echo prelude-marker")

	child.await(t, "echo output", func(screen string) bool {
		// The echoed command line plus the line the shell printed.
		return strings.Count(screen, "prelude-marker") >= 2
	})
}

func TestChildFullScreenClearCannotReachPinnedRows(t *testing.T) {
	// This is the architectural claim in one test. The child emits the most
	// destructive thing it can - erase display, home the cursor - and the
	// pinned rows of the composed frame stay intact, because the child never
	// addresses the outer terminal at all.
	child := startTestShell(t, 40, 4)
	child.run("echo before-clear")
	child.await(t, "pre-clear output", func(screen string) bool {
		return strings.Contains(screen, "before-clear")
	})

	frame := uv.NewScreenBuffer(40, 8)
	pinned := textSurface([]string{"PINNED ROW A", "PINNED ROW B"}, uv.Style{}, 40, 2)
	pinned.blit(&frame, 0, 0)

	child.run(`printf '\033[2J\033[H'; echo after-clear`)
	child.await(t, "the screen to be cleared and repainted", func(screen string) bool {
		return strings.Contains(screen, "after-clear") && !strings.Contains(screen, "before-clear")
	})

	// Compose the child into the rows below the pin, exactly as the host does.
	child.draw(&frame, band{top: 2, height: 4}, 0, blankPaint)

	if got := rowText(&frame, 0); !strings.HasPrefix(got, "PINNED ROW A") {
		t.Fatalf("pinned row 0 = %q, want it intact after the child cleared its screen", got)
	}
	if got := rowText(&frame, 1); !strings.HasPrefix(got, "PINNED ROW B") {
		t.Fatalf("pinned row 1 = %q, want it intact after the child cleared its screen", got)
	}
	if got := rowText(&frame, 2); strings.Contains(got, "PINNED") {
		t.Fatalf("shell band row = %q, want child cells, not leftover pin text", got)
	}
}

func TestChildAlternateScreenIsReportedAndContained(t *testing.T) {
	child := startTestShell(t, 40, 4)
	if child.view().altScreen {
		t.Fatal("a fresh shell must start on the main screen")
	}

	child.run(`printf '\033[?1049h'`)
	child.await(t, "alternate screen entry", func(string) bool { return child.view().altScreen })

	child.run(`printf '\033[?1049l'`)
	child.await(t, "alternate screen exit", func(string) bool { return !child.view().altScreen })
}

func TestResizeReshapesTheChildScreen(t *testing.T) {
	child := startTestShell(t, 40, 4)

	changed, err := child.resize(64, 9)
	if err != nil {
		t.Fatalf("resize: %v", err)
	}
	if !changed {
		t.Fatal("resize to a new geometry reported no change")
	}
	if view := child.view(); view.cols != 64 || view.rows != 9 {
		t.Fatalf("view geometry = %dx%d, want 64x9", view.cols, view.rows)
	}

	child.run("stty size")
	child.await(t, "the child to observe its new size", func(screen string) bool {
		return strings.Contains(screen, "9 64")
	})
}

func TestRepeatedResizeToTheSameGeometryIsANoOp(t *testing.T) {
	// The host leans on this to avoid a SIGWINCH storm while a window edge is
	// being dragged.
	child := startTestShell(t, 40, 4)

	if changed, err := child.resize(40, 4); err != nil || changed {
		t.Fatalf("resize(same) = (%t, %v), want (false, nil)", changed, err)
	}
}

func TestNormalExitReportsItsStatus(t *testing.T) {
	child := startTestShell(t, 40, 4)

	child.run("exit 3")

	select {
	case <-child.exited:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the child to exit")
	}

	if code := child.exitCode(); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
}

func TestScrollbackExposesEvictedLines(t *testing.T) {
	child := startTestShell(t, 40, 4)

	child.run("for i in 1 2 3 4 5 6 7 8 9; do echo line-$i; done")
	child.await(t, "the loop to finish", func(screen string) bool {
		return strings.Contains(screen, "line-9")
	})

	history := child.view().history
	if history == 0 {
		t.Fatal("lines scrolled off the 4-row screen but no scrollback was retained")
	}
	if strings.Contains(child.text(), "line-1\n") {
		t.Fatal("line-1 is still on the live screen; the test proves nothing")
	}

	// The evicted line must be reachable at some scroll offset. Which offset
	// depends on how the prompt and the echoed command wrapped, so the test
	// asserts reachability rather than a hard-coded position.
	frame := uv.NewScreenBuffer(40, 4)
	for offset := 0; offset <= history; offset++ {
		child.draw(&frame, band{top: 0, height: 4}, offset, blankPaint)
		if strings.Contains(composedText(&frame), "line-1") {
			return
		}
	}

	child.draw(&frame, band{top: 0, height: 4}, history, blankPaint)
	t.Fatalf("line-1 was not reachable at any of the %d scroll offsets; top of history:\n%s",
		history, composedText(&frame))
}

// rowText flattens one composed frame row for assertions.
func rowText(frame *uv.ScreenBuffer, y int) string {
	var row strings.Builder
	for x := range frame.Width() {
		if cell := frame.CellAt(x, y); cell != nil {
			row.WriteString(cell.Content)
		}
	}
	return strings.TrimRight(row.String(), " ")
}

// composedText flattens a whole composed frame for assertions.
func composedText(frame *uv.ScreenBuffer) string {
	var out strings.Builder
	for y := range frame.Height() {
		out.WriteString(rowText(frame, y))
		out.WriteByte('\n')
	}
	return out.String()
}
