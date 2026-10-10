package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestHostCompletionDelayedQueryCannotAcknowledgeApply(t *testing.T) {
	if os.Getenv("PRELUDE_COMPLETION_INIT") == "" {
		t.Fatal("real generated completion init is required; run in nix develop path:.#ghostty-spike")
	}
	runLog := filepath.Join(t.TempDir(), "x.executions")
	h := newSpikeTestHost(t, "GHOSTTY_COMPLETION_EXECUTION_LOG="+runLog,
		`BASH_FUNC_x%%=() { if [[ ${1-} == --imports ]]; then command x "$@"; else builtin printf '%s\n' "$*" >> "$GHOSTTY_COMPLETION_EXECUTION_LOG"; return 23; fi; }`)
	h.Update(tea.PasteMsg{Content: "true; printf 'COMPLETION_INITIAL_STATUS:%s\\n' \"$?\""})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "initial command completed", func() bool {
		return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), "COMPLETION_INITIAL_STATUS:0")
	})
	h.Update(tea.PasteMsg{Content: "x go:"})
	awaitSpike(t, h, "query input", func() bool { return spikeInput(h) == "x go:" })
	spikeKey(h, tea.KeyTab, 0)

	// Capture actual notification bytes while driving the existing host loop.
	// Do not invent snapshot IDs, read private files, or inspect apply flags.
	var output []byte
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for !strings.Contains(ansi.Strip(h.View().Content), completionHint) {
		select {
		case msg := <-h.child.output:
			output = append(output, msg.data...)
			h.Update(msg)
		case <-h.child.exited:
			t.Fatalf("Bash exited before query response: %v", h.child.waitErr)
		case <-timer.C:
			t.Fatalf("waiting for real query response; footer=%q\n%s", h.footer(), spikeFrameText(h.frame))
		}
		if h.err != nil {
			t.Fatal(h.err)
		}
	}
	markers := regexp.MustCompile(`\x1b\]133;Q(?:;[^\x07\x1b]*)?(?:\x07|\x1b\\)`).FindAll(output, -1)
	if len(markers) == 0 {
		t.Fatal("visible completion did not include an actual Q notification")
	}
	oldQuery := bytes.Clone(markers[len(markers)-1])
	view := ansi.Strip(h.View().Content)
	var key string
	for _, row := range strings.Split(view, "\n") {
		if selected := strings.TrimSpace(row); strings.HasPrefix(selected, "> ") {
			words := strings.Fields(strings.TrimPrefix(selected, "> "))
			if len(words) > 0 {
				key = words[0]
			}
		}
	}
	if key == "" {
		t.Fatal("visible selected completion marker missing")
	}

	spikeKey(h, tea.KeyEnter, 0)
	// Withhold the real apply acknowledgement. A delayed/duplicate Q must not
	// clear acceptance-in-progress and turn the following Enter into execution.
	h.Update(outputMsg{data: oldQuery})
	h.Update(outputMsg{data: []byte("\x1b]133;Q\a")})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "only the real apply acknowledgement releases acceptance", func() bool {
		return h.prompt.ready && strings.TrimSpace(spikeInput(h)) == "x "+key &&
			!strings.Contains(ansi.Strip(h.View().Content), completionHint)
	})
	// A later edit still targets the completed line; no old Q may replace it.
	h.Update(tea.PasteMsg{Content: "kept"})
	awaitSpike(t, h, "completed readline remains editable after delayed Q", func() bool {
		return h.prompt.ready && strings.TrimSpace(spikeInput(h)) == "x "+key+" kept"
	})
	if _, err := os.Stat(runLog); !os.IsNotExist(err) {
		t.Fatalf("a delayed Q allowed Enter to execute the accepted command: %v", err)
	}
}

func TestHostCompletionOldFallbackDoesNotDisarmNewerTab(t *testing.T) {
	if os.Getenv("PRELUDE_COMPLETION_INIT") == "" {
		t.Fatal("real generated completion init is required; run in nix develop path:.#ghostty-spike")
	}
	realX, err := exec.LookPath("x")
	if err != nil {
		t.Fatal(err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ready, release := filepath.Join(dir, "imports.ready"), filepath.Join(dir, "imports.release")
	// Gate the public imports invocation, forwarding everything else to real x.
	// The newer query cannot publish until the old fallback has been delivered.
	wrapper := fmt.Sprintf("#!%s -p\nif [[ ${1-} == --imports ]]; then\n  : > %q\n  while [[ ! -e %q ]]; do sleep 0.01; done\nfi\nexec %q \"$@\"\n", bash, ready, release, realX)
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	defer os.WriteFile(release, nil, 0o600)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	runLog := filepath.Join(dir, "x.executions")
	h := newSpikeTestHost(t, "GHOSTTY_COMPLETION_EXECUTION_LOG="+runLog,
		`BASH_FUNC_x%%=() { if [[ ${1-} == --imports ]]; then command x "$@"; else builtin printf '%s\n' "$*" >> "$GHOSTTY_COMPLETION_EXECUTION_LOG"; return 23; fi; }`)
	h.Update(tea.PasteMsg{Content: "true; printf 'COMPLETION_INITIAL_STATUS:%s\\n' \"$?\""})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "initial command completed", func() bool {
		return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), "COMPLETION_INITIAL_STATUS:0")
	})
	h.Update(tea.PasteMsg{Content: "echo old-fallback-no-match"})
	awaitSpike(t, h, "native fallback input", func() bool { return spikeInput(h) == "echo old-fallback-no-match" })
	// While the outer terminal is blurred, Tab is forwarded without a tracked
	// host request, but Readline still produces a real server fallback snapshot.
	h.Update(tea.BlurMsg{})
	spikeKey(h, tea.KeyTab, 0)
	var held []byte
	query := regexp.MustCompile(`\x1b\]133;Q(?:;[^\x07\x1b]*)?(?:\x07|\x1b\\)`)
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for !query.Match(held) {
		select {
		case msg := <-h.child.output:
			held = append(held, msg.data...)
		case <-h.child.exited:
			t.Fatalf("Bash exited before old fallback: %v", h.child.waitErr)
		case <-timer.C:
			t.Fatal("waiting for the real untracked fallback Q")
		}
	}
	h.Update(tea.FocusMsg{})
	spikeKey(h, 'u', tea.ModCtrl)
	h.Update(tea.PasteMsg{Content: "x go:"})
	spikeKey(h, tea.KeyTab, 0)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		} else if !os.IsNotExist(err) || time.Now().After(deadline) {
			t.Fatalf("newer query did not reach the public imports gate: %v", err)
		}
		time.Sleep(outputPace)
	}
	// No query/apply file or wanted flag is read: gate ordering guarantees this
	// is still the older fallback, even though a newer Tab is now tracked.
	h.Update(outputMsg{data: held})
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	awaitSpike(t, h, "newer candidates open after the unseen old fallback", func() bool {
		return h.prompt.ready && spikeInput(h) == "x go:" && strings.Contains(ansi.Strip(h.View().Content), completionHint)
	})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "newer popup accepts without executing", func() bool {
		return h.prompt.ready && strings.TrimSpace(spikeInput(h)) == "x go:test" && !strings.Contains(ansi.Strip(h.View().Content), completionHint)
	})
	if _, err := os.Stat(runLog); !os.IsNotExist(err) {
		t.Fatalf("accepting the newer popup executed its candidate: %v", err)
	}
}

func TestHostCompletionReflowInvisibleEnterRunsOriginalInput(t *testing.T) {
	if os.Getenv("PRELUDE_COMPLETION_INIT") == "" {
		t.Fatal("real generated completion init is required; run in nix develop path:.#ghostty-spike")
	}
	runLog := filepath.Join(t.TempDir(), "x.args")
	// Only execution is a probe; queries still use real x's imports and the
	// generated catalogue. An invisible acceptance would swallow this Enter.
	h := newSpikeTestHost(t, "GHOSTTY_COMPLETION_GEOMETRY_LOG="+runLog,
		`BASH_FUNC_x%%=() { if [[ ${1-} == --imports ]]; then command x "$@"; else builtin printf '%s\n' "$*" > "$GHOSTTY_COMPLETION_GEOMETRY_LOG"; return 23; fi; }`)
	h.Update(tea.PasteMsg{Content: "x go:"})
	awaitSpike(t, h, "input before reflow", func() bool { return spikeInput(h) == "x go:" })
	spikeKey(h, tea.KeyTab, 0)
	awaitSpike(t, h, "visible chooser before reflow", func() bool {
		return spikeInput(h) == "x go:" && strings.Contains(ansi.Strip(h.View().Content), completionHint)
	})
	h.Update(tea.WindowSizeMsg{Width: 60, Height: 2})
	awaitSpike(t, h, "reflow hides a chooser with no safe rows", func() bool {
		return spikeInput(h) == "x go:" && !strings.Contains(ansi.Strip(h.View().Content), completionHint) && h.frame.Cols == 60 && h.frame.Rows == 1
	})
	if _, err := os.Stat(runLog); !os.IsNotExist(err) {
		t.Fatalf("resize executed a completion: %v", err)
	}
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "Enter executes the unchanged line, not an invisible selection", func() bool {
		data, err := os.ReadFile(runLog)
		return err == nil && string(data) == "go:\n" && h.prompt.ready && spikeInput(h) == ""
	})
	spikeShellStatus(t, h, 23)
	if frontendPTYSize(t, h.child) != [2]int{60, 1} || h.frame.Cols != 60 || h.frame.Rows != 1 {
		t.Fatal("invisible chooser acceptance resized Bash or Ghostty")
	}
}

func TestHostCompletionFloatingPaneDoesNotCoverProjectedPrompt(t *testing.T) {
	if os.Getenv("PRELUDE_COMPLETION_INIT") == "" {
		t.Fatal("real generated completion init is required; run in nix develop path:.#ghostty-spike")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wrapper := fmt.Sprintf("#!%s -p\nexport GHOSTTY_FRONTEND_TOOL=DOCS\nexec %q -test.run=^TestFrontendToolHelper$\n", bash, helper)
	if err := os.WriteFile(filepath.Join(dir, "docs"), []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	h := newSpikeTestHost(t)
	h.Update(tea.PasteMsg{Content: "printf 'FLOAT_FILL\\n%.0s' {1..48}"})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "prompt at the bottom of the native viewport", func() bool {
		return h.prompt.ready && spikeInput(h) == "" && h.frame.CursorY == h.frame.Rows-1 && strings.Contains(spikeFrameText(h.frame), "FLOAT_FILL")
	})
	frontendChord(h, 'd')
	awaitFrontendReady(t, h, "DOCS")
	frontendChord(h, tea.KeyTab)
	h.Update(tea.PasteMsg{Content: "x go:"})
	awaitSpike(t, h, "visible shell input below floating docs", func() bool { return spikeInput(h) == "x go:" })
	mainSize, paneSize := frontendPTYSize(t, h.child), frontendPTYSize(t, h.pane.process)
	layoutBefore := h.layout()
	nativeText := spikeFrameText(h.frame)
	spikeKey(h, tea.KeyTab, 0)
	awaitHostCompletionQuery(t, h)
	awaitSpike(t, h, "Readline redraw after the occluded completion query", func() bool { return spikeInput(h) == "x go:" })
	view := h.View()
	rows := strings.Split(ansi.Strip(view.Content), "\n")
	if strings.Contains(ansi.Strip(view.Content), completionHint) || len(rows) != h.rows ||
		strings.TrimSpace(rows[h.frame.Rows-1]) != strings.TrimSpace(spikePrompt+"x go:") ||
		view.Cursor == nil || view.Cursor.Y != h.frame.CursorY || spikeFrameText(h.frame) != nativeText {
		t.Fatalf("completion moved the prompt/input beneath floating docs\n%s", ansi.Strip(view.Content))
	}
	layout := h.layout()
	if frontendPTYSize(t, h.child) != mainSize || frontendPTYSize(t, h.pane.process) != paneSize ||
		layout.Shell != layoutBefore.Shell || layout.Panel != layoutBefore.Panel || layout.Body != layoutBefore.Body {
		t.Fatal("floating-pane suppression changed native or pane geometry")
	}
	// Removing the occluder must make the same line completable again.
	frontendChord(h, 'c')
	spikeKey(h, tea.KeyTab, 0)
	awaitSpike(t, h, "closing floating docs makes completion usable", func() bool {
		return spikeInput(h) == "x go:" && strings.Contains(ansi.Strip(h.View().Content), completionHint)
	})
	spikeKey(h, tea.KeyEscape, 0)
	if spikeInput(h) != "x go:" || h.footer() != baseFooter {
		t.Fatal("floating-pane completion cleanup changed input or the existing footer")
	}
}

func TestHostCompletionMouseFocusKeepsNativeGeometry(t *testing.T) {
	if os.Getenv("PRELUDE_COMPLETION_INIT") == "" {
		t.Fatal("real generated completion init is required; run in nix develop path:.#ghostty-spike")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	helper, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	wrapper := fmt.Sprintf("#!%s -p\nexport GHOSTTY_FRONTEND_TOOL=DOCS\nexec %q -test.run=^TestFrontendToolHelper$\n", bash, helper)
	if err := os.WriteFile(filepath.Join(dir, "docs"), []byte(wrapper), 0o700); err != nil {
		t.Fatal(err)
	}
	// Keep real x and generated completion; only the mouse-reporting docs pane
	// is a fixture. Dock it so the click target cannot overlap the popup.
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	h := newSpikeTestHost(t)
	h.Update(tea.PasteMsg{Content: "true; printf 'COMPLETION_INITIAL_STATUS:%s\\n' \"$?\""})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "initial command completed", func() bool {
		return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), "COMPLETION_INITIAL_STATUS:0")
	})
	frontendChord(h, 'd')
	awaitFrontendReady(t, h, "DOCS")
	spikeKey(h, ']', tea.ModCtrl)
	frontendChord(h, tea.KeyTab)
	h.Update(tea.PasteMsg{Content: "x go:"})
	awaitSpike(t, h, "shell input with visible docked pane", func() bool { return spikeInput(h) == "x go:" })
	before := frontendPTYSize(t, h.child)
	paneBefore := frontendPTYSize(t, h.pane.process)
	layoutBefore := h.layout()
	spikeKey(h, tea.KeyTab, 0)
	awaitFrontend(t, h, "display-only popup leaves shell and pane geometry unchanged", func() bool {
		return strings.Contains(ansi.Strip(h.View().Content), completionHint)
	})
	assertGeometry := func() {
		t.Helper()
		layout := h.layout()
		if frontendPTYSize(t, h.child) != before || frontendPTYSize(t, h.pane.process) != paneBefore ||
			h.frame.Cols != before[0] || h.frame.Rows != before[1] || h.pane.frame.Cols != paneBefore[0] || h.pane.frame.Rows != paneBefore[1] ||
			layout.Shell != layoutBefore.Shell || layout.Body != layoutBefore.Body || layout.Panel != layoutBefore.Panel || layout.FooterY != layoutBefore.FooterY {
			t.Fatal("completion overlay changed shell, engine, docked pane, or sidebar geometry")
		}
	}
	assertGeometry()
	spikeKey(h, tea.KeyTab, 0)
	assertGeometry()
	h.Update(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	awaitFrontend(t, h, "mouse focus closes popup without changing native geometry", func() bool {
		return !strings.Contains(ansi.Strip(h.View().Content), completionHint) &&
			frontendPTYSize(t, h.child) == before && frontendPTYSize(t, h.pane.process) == paneBefore
	})
	assertGeometry()
	h.Update(tea.PasteMsg{Content: "mouse-owned"})
	awaitFrontend(t, h, "clicked pane owns input rather than the old popup", func() bool {
		return strings.Contains(frontendCompactText(h.pane.frame), fmt.Sprintf("%x", "mouse-owned")) && spikeInput(h) == "x go:"
	})
	frontendChord(h, 'c')
	awaitSpike(t, h, "closing pane preserves original readline input", func() bool { return h.prompt.ready && spikeInput(h) == "x go:" })
}
