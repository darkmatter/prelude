package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func spikeFrame(cols, rows int) terminalFrame {
	frame := terminalFrame{Cols: cols, Rows: rows, Cells: make([]uv.Cell, cols*rows), CursorVisible: true}
	for i := range frame.Cells {
		frame.Cells[i] = uv.EmptyCell
	}
	return frame
}

func spikeFrameText(frame terminalFrame) string {
	var text strings.Builder
	for y := range frame.Rows {
		for x := range frame.Cols {
			text.WriteString(frame.Cells[y*frame.Cols+x].Content)
		}
		text.WriteByte('\n')
	}
	return text.String()
}

// Reconstruct only the current native display for readline assertions. This
// helper never mutates the semantic anchor or retains an editable input buffer.
func spikeInput(h *host) string {
	frame, prompt := h.frame, &h.prompt
	if !prompt.ready || frame.AltScreen || frame.Cols < 1 || prompt.anchor < 0 ||
		prompt.anchorCols != frame.Cols || !prompt.promptAt(frame, prompt.anchor) {
		return "(input not visible)"
	}
	cursor := frame.CursorY*frame.Cols + frame.CursorX
	if prompt.anchor > cursor || cursor > len(frame.Cells) {
		return "(input not visible)"
	}
	end := max(prompt.anchor, cursor)
	for i := prompt.anchor; i < len(frame.Cells); i++ {
		cell := frame.Cells[i]
		if cell.Width > 0 && cell.Content != "" && cell.Content != " " {
			end = max(end, i+cell.Width)
		}
	}
	var input strings.Builder
	for _, cell := range frame.Cells[prompt.anchor:min(end, len(frame.Cells))] {
		if cell.Width <= 0 {
			continue
		}
		if cell.Content == "" {
			input.WriteByte(' ')
		} else {
			input.WriteString(cell.Content)
		}
	}
	return input.String()
}

// Capture Bash's real status as command output, including in a one-row viewport
// where the next prompt would immediately scroll a printed status off-screen.
func spikeShellStatus(t *testing.T, h *host, want int) {
	t.Helper()
	if !h.prompt.ready || spikeInput(h) != "" {
		t.Fatalf("status probe requires an empty primary prompt: phase=%q input=%q", h.prompt.phase, spikeInput(h))
	}
	path := filepath.Join(t.TempDir(), "shell-status")
	h.Update(tea.PasteMsg{Content: fmt.Sprintf("printf '%%s\\n' \"$?\" > %q", path)})
	spikeKey(h, tea.KeyEnter, 0)
	var output []byte
	awaitSpike(t, h, "real Bash status output", func() bool {
		var err error
		output, err = os.ReadFile(path)
		return err == nil && h.prompt.ready && !h.shellAcceptPending && spikeInput(h) == ""
	})
	if got := string(output); got != fmt.Sprintf("%d\n", want) {
		t.Fatalf("Bash status output = %q, want %d", got, want)
	}
}

func spikeFooterKeyText(hints []keyHint) string {
	var groups []string
	for _, hint := range hints {
		group := " " + hint.key + " "
		if hint.text != "" {
			group += " " + hint.text
		}
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		return ""
	}
	return "  " + strings.Join(groups, "  ")
}

func assertSpikeFooter(t *testing.T, h *host, keys, status string) string {
	t.Helper()
	want := keys
	if status != "" {
		statusX := h.cols - 2 - ansi.StringWidth(status)
		if keys != "" && ansi.StringWidth(keys)+2 > statusX {
			t.Fatalf("expected hints and status overlap: keys=%q status=%q cols=%d", keys, status, h.cols)
		}
		want += strings.Repeat(" ", max(0, statusX-ansi.StringWidth(keys))) + status
	}
	rows := strings.Split(ansi.Strip(h.View().Content), "\n")
	if len(rows) != h.rows || h.layout().FooterY != h.rows-1 {
		t.Fatalf("protected footer geometry changed: rows=%d layout=%+v", len(rows), h.layout())
	}
	row := strings.TrimRight(rows[h.rows-1], " ")
	if row != want || h.footer() != want {
		t.Fatalf("protected footer row = %q (text=%q), want %q", row, h.footer(), want)
	}
	return row
}

func TestHostFooterKeepsOperationalHintsInProtectedRow(t *testing.T) {
	h := &host{cols: 120, rows: 8, frame: spikeFrame(120, 7), terminalFocused: true}
	for _, phase := range []string{"", "prompt", "running", "exited"} {
		h.prompt.phase, h.prompt.ready = phase, phase == "prompt"
		assertSpikeFooter(t, h, "   LOCKED    Ctrl+P  unlock", "")
	}
	h.notice, h.pendingCommand = "Fixture notice", "echo queued"
	assertChromeFooter(t, h, spikeNavigationKeys, "Fixture notice | command queued", h.palette.Warning)
	layout := h.layout()
	rows := strings.Split(ansi.Strip(h.View().Content), "\n")
	if strings.TrimSpace(strings.Join(rows[:layout.FooterY], "")) != "" ||
		layout.Shell.Dx() != h.frame.Cols || layout.Shell.Dy() != h.frame.Rows {
		t.Fatalf("notice/queue content escaped the single protected footer or changed native geometry: layout=%+v rows=%q", layout, rows)
	}
}

func TestHostViewProtectsFooterAndRestoresHiddenPane(t *testing.T) {
	h := &host{cols: 70, rows: 8, frame: spikeFrame(70, 7), surface: surfaceDocs, terminalFocused: true}
	h.paneVisible = true
	body := h.layout().Body
	p := &pane{kind: surfaceDocs, frame: spikeFrame(body.Dx(), body.Dy())}
	p.frame.Cells[0] = uv.Cell{Content: "D", Width: 1}
	h.pane, h.paneVisible = p, false
	h.frame.Cells[0] = uv.Cell{Content: "界", Width: 2}
	h.frame.Cells[1] = uv.Cell{}
	h.frame.Cells[2] = uv.Cell{Content: "e\u0301", Width: 1}
	h.frame.CursorX, h.frame.CursorY = 33, 3
	h.frame.CursorStyle, h.frame.CursorBlink = 1, true
	h.frame.Title, h.frame.MouseTracking = "child title", true
	original := h.View()
	lines := strings.Split(ansi.Strip(original.Content), "\n")
	if len(lines) != h.rows || !strings.HasPrefix(lines[0], "界e\u0301") {
		t.Fatalf("composed frame = %q", lines)
	}
	assertSpikeFooter(t, h, spikeFooterKeyText(spikeHiddenKeys), "docs hidden | running")
	if original.Cursor == nil || original.Cursor.X != 33 || original.Cursor.Y != 3 || original.Cursor.Shape != tea.CursorBar || !original.Cursor.Blink {
		t.Fatalf("cursor = %+v", original.Cursor)
	}
	if !original.AltScreen || !original.ReportFocus || original.WindowTitle != "child title" || original.MouseMode != tea.MouseModeAllMotion {
		t.Fatalf("view properties = %+v", original)
	}
	cells, paneCells := append([]uv.Cell(nil), h.frame.Cells...), append([]uv.Cell(nil), p.frame.Cells...)
	h.paneVisible = true
	shown := h.View()
	shownLines := strings.Split(ansi.Strip(shown.Content), "\n")
	if strings.TrimSpace(shownLines[body.Min.Y]) != "D" || shown.Cursor != nil ||
		!strings.Contains(h.footer(), "docs | floating | focus:shell | running") {
		t.Fatalf("borderless docs did not begin at the panel origin, cover the shell cursor, or report footer status: %+v", shown)
	}
	if content := strings.Join(shownLines[:h.rows-1], "\n"); strings.Contains(content, "docs |") || strings.ContainsAny(content, "┌┐└┘│─") {
		t.Fatalf("host added a border/title strip outside the footer: %q", content)
	}
	assertSpikeFooter(t, h, spikeFooterKeyText(spikeNavigationKeys), "docs | floating | focus:shell | running")
	if !reflect.DeepEqual(h.frame.Cells, cells) || !reflect.DeepEqual(p.frame.Cells, paneCells) {
		t.Fatal("docs window changed a child snapshot")
	}
	h.paneVisible = false
	if restored := h.View(); restored.Content != original.Content || !reflect.DeepEqual(restored.Cursor, original.Cursor) {
		t.Fatal("hiding docs did not restore the exact shell screen and cursor")
	}
	assertSpikeFooter(t, h, spikeFooterKeyText(spikeHiddenKeys), "docs hidden | running")
	if h.pane != p {
		t.Fatal("hidden docs did not retain its pane and show hint")
	}
	h.frame.CursorVisible = false
	if h.View().Cursor != nil {
		t.Fatal("host showed a cursor hidden by the child")
	}
}

func TestHostViewFitsTinyGeometry(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {5, 2}, {13, 3}, {15, 4}} {
		cols, rows := size[0], size[1]
		h := &host{cols: cols, rows: rows, frame: spikeFrame(cols, shellRows(rows)), terminalFocused: true, surface: surfaceDocs, paneVisible: true}
		h.frame.Cells[0] = uv.Cell{Content: "S", Width: 1}
		lines := strings.Split(ansi.Strip(h.View().Content), "\n")
		if len(lines) != rows {
			t.Fatalf("%dx%d painted %d rows: %q", cols, rows, len(lines), lines)
		}
		for _, line := range lines {
			if ansi.StringWidth(line) > cols {
				t.Fatalf("%dx%d overflowed with %q", cols, rows, line)
			}
		}
		if rows == 1 && strings.TrimSpace(lines[0]) != "S" {
			t.Fatal("one-row terminal must not sacrifice its only native child row for a footer")
		}
	}
}

func newSpikeTestHost(t *testing.T, extraEnv ...string) *host {
	t.Helper()
	isolatedMotd(t)
	// The Nix builder starts in the C locale; readline needs a UTF-8 locale
	// for this fixture's Unicode input, independently of the user's shell.
	locale := "C.UTF-8"
	if runtime.GOOS == "darwin" {
		locale = "en_US.UTF-8"
	}
	t.Setenv("LC_ALL", locale)
	h, err := newHost(84, 12, append(os.Environ(), extraEnv...), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.close(); err != nil {
			t.Error(err)
		}
	})
	awaitSpike(t, h, "initial semantic prompt", func() bool { return h.prompt.ready && spikeInput(h) == "" })
	return h
}

// Tests drive the same Update interface as Bubble Tea. Only the production
// reader/waiter are concurrent; all engine calls remain on this test's UI loop.
func awaitSpike(t *testing.T, h *host, what string, check func() bool) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		if h.err != nil {
			t.Fatalf("%s: %v", what, h.err)
		}
		if check() {
			return
		}
		select {
		case message := <-h.child.output:
			h.Update(message)
		case <-h.child.exited:
			// Take queued final bytes before observing the wait result.
			select {
			case message := <-h.child.output:
				h.Update(message)
				continue
			default:
			}
			h.Update(childExitMsg{err: h.child.waitErr})
			if check() {
				return
			}
			t.Fatalf("Bash exited before %s: code=%d\n%s", what, h.code, spikeFrameText(h.frame))
		case <-timer.C:
			t.Fatalf("waiting for %s: phase=%q ready=%t anchor=%d anchorCols=%d promptRows=%d measuredCols=%d input=%q footer=%q\n%s", what, h.prompt.phase, h.prompt.ready, h.prompt.anchor, h.prompt.anchorCols, h.prompt.promptRows, h.prompt.promptRowsCols, spikeInput(h), h.footer(), spikeFrameText(h.frame))
		}
	}
}

func spikeKey(h *host, code rune, mod tea.KeyMod) {
	h.Update(tea.KeyPressMsg{Code: code, Mod: mod})
}

func TestHostRealBashReadlineAndSemanticLifecycle(t *testing.T) {
	frontendTools(t)
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("echo USER_RC_RAN\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newSpikeTestHost(t, "HOME="+home, "SHELLOPTS=braceexpand:hashall:interactive-comments:nounset:functrace",
		"BASH_FUNC_spike_tool%%=() { builtin printf 'DEV_SHELL_TOOL\\n'; }")
	if strings.Contains(spikeFrameText(h.frame), "USER_RC_RAN") {
		t.Fatal("private Bash loaded a user rc")
	}
	if cursor := h.View().Cursor; cursor == nil || cursor.Shape != tea.CursorBar || !cursor.Blink {
		t.Fatalf("Bash cursor must be a blinking bar: %+v", cursor)
	}
	h.Update(tea.PasteMsg{Content: "printf 'history-ok\\n'"})
	awaitSpike(t, h, "paste in actual readline line", func() bool { return spikeInput(h) == "printf 'history-ok\\n'" })
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "completed command", func() bool {
		return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), "\nhistory-ok")
	})
	spikeKey(h, tea.KeyUp, 0)
	awaitSpike(t, h, "readline history", func() bool { return spikeInput(h) == "printf 'history-ok\\n'" })
	spikeKey(h, 'a', tea.ModCtrl)
	spikeKey(h, tea.KeyDelete, 0)
	awaitSpike(t, h, "editing before the cursor with a suffix", func() bool { return spikeInput(h) == "rintf 'history-ok\\n'" })
	spikeKey(h, 'k', tea.ModCtrl)
	h.Update(tea.PasteMsg{Content: "spike_too"})
	spikeKey(h, tea.KeyTab, 0)
	awaitSpike(t, h, "completion of an inherited devshell function", func() bool { return spikeInput(h) == "spike_tool " })
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "real inherited tool output", func() bool {
		return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), "DEV_SHELL_TOOL")
	})
	h.Update(tea.PasteMsg{Content: "sleep 0.3; false"})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "OSC133 running state", func() bool {
		return h.prompt.phase == "running" && !h.prompt.ready && strings.Contains(spikeFrameText(h.frame), "sleep 0.3; false")
	})
	awaitSpike(t, h, "OSC133 returns to an editable primary prompt", func() bool {
		return h.prompt.ready && spikeInput(h) == ""
	})
	spikeShellStatus(t, h, 1)
	spikeKey(h, 'x', tea.ModAlt)
	awaitFrontendReady(t, h, "MENU")
	p := h.pane
	if !h.paneVisible || !h.paneFocused() || h.placement != placementFloating {
		t.Fatal("Alt+x did not open and focus the floating menu")
	}
	frontendChord(h, 't')
	h.Update(tea.PasteMsg{Content: "echo 界"})
	awaitSpike(t, h, "typing in Bash with the menu hidden", func() bool { return spikeInput(h) == "echo 界" })
	spikeKey(h, 'v', tea.ModCtrl)
	spikeKey(h, 'g', tea.ModCtrl)
	awaitSpike(t, h, "quoted reserved key reaching readline", func() bool { return strings.Contains(spikeInput(h), "^G") })
	if h.paneVisible || h.pane != p || !h.currentPane(p) || h.focusPane {
		t.Fatal("quoted Ctrl+G incorrectly showed or replaced the hidden menu")
	}
	spikeKey(h, 'c', tea.ModCtrl)
	awaitSpike(t, h, "cancelled readline input", func() bool { return h.prompt.ready && spikeInput(h) == "" })
	spikeShellStatus(t, h, 130)
	h.Update(tea.PasteMsg{Content: "printf 'spike $ literal'"})
	awaitSpike(t, h, "prompt text inside actual input", func() bool { return spikeInput(h) == "printf 'spike $ literal'" })
}

func TestHostRestoresBlinkingCursorWhenBashRegainsInput(t *testing.T) {
	frontendTools(t)
	home := t.TempDir()
	config := filepath.Join(home, "starship.toml")
	if err := os.WriteFile(config, []byte(smokeStarshipConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, starship := range []bool{false, true} {
		name := "FixedBash"
		if starship {
			name = "Starship"
		}
		t.Run(name, func(t *testing.T) {
			env := append(smokeEnvironment(home), "STARSHIP_CONFIG="+config)
			h, err := newHost(120, 16, env, starship)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := h.close(); err != nil {
					t.Error(err)
				}
			})
			awaitSpike(t, h, "initial prompt", func() bool { return h.prompt.ready && spikeInput(h) == "" })

			command := "printf '\\033[6 q\\033[?12l'; sleep 0.2; false"
			h.Update(tea.PasteMsg{Content: command})
			awaitSpike(t, h, "command input", func() bool { return spikeInput(h) == command })
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "running child sets a steady cursor", func() bool {
				return h.prompt.phase == "running" && !h.frame.CursorBlink
			})
			if cursor := h.View().Cursor; cursor == nil || cursor.Blink {
				t.Fatalf("running child must retain its steady cursor: %+v", cursor)
			}
			awaitSpike(t, h, "returned prompt", func() bool { return h.prompt.ready && spikeInput(h) == "" })
			if cursor := h.View().Cursor; cursor == nil || cursor.Shape != tea.CursorBar || !cursor.Blink {
				t.Fatalf("Bash should restore a blinking bar after a child changes it: %+v", cursor)
			}
			spikeShellStatus(t, h, 1)
		})
	}
}

func TestHostLockFooterAcrossPromptModes(t *testing.T) {
	frontendTools(t)
	home := t.TempDir()
	custom := filepath.Join(home, "starship.toml")
	if err := os.WriteFile(custom, []byte(smokeStarshipConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, config string
		starship     bool
	}{
		{"FixedBash", custom, false},
		{"CustomStarship", custom, true},
		{"DefaultStarship", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := append(smokeEnvironment(home), "STARSHIP_CONFIG="+tc.config)
			h, err := newHost(120, 16, env, tc.starship)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := h.close(); err != nil {
					t.Error(err)
				}
			})
			awaitSpike(t, h, "editable prompt in locked mode", func() bool { return h.prompt.ready && spikeInput(h) == "" })
			keys := spikeFooterKeyText(spikeNavigationKeys)
			assertChromeFooter(t, h, spikeNavigationKeys, "", nil)
			input := "echo locked-界é🙂"
			h.Update(tea.PasteMsg{Content: input})
			awaitSpike(t, h, "real editable input with locked footer", func() bool { return spikeInput(h) == input })
			native := ghosttySnapshot(t, h.terminal)
			view := h.View()
			if frontendPTYSize(t, h.child) != [2]int{120, 15} || h.frame.Cols != 120 || h.frame.Rows != 15 ||
				view.Cursor == nil || view.Cursor.X != native.CursorX || view.Cursor.Y != native.CursorY {
				t.Fatalf("locked footer changed real input/PTY/caret geometry: cursor=%+v\n%s", view.Cursor, ansi.Strip(view.Content))
			}
			for _, cancel := range []rune{tea.KeyEscape, 'p'} {
				spikeKey(h, 'p', tea.ModCtrl)
				assertChromeFooter(t, h, spikeLeaderKeys, "", nil)
				mod := tea.KeyMod(0)
				if cancel == 'p' {
					mod = tea.ModCtrl
				}
				spikeKey(h, cancel, mod)
				assertSpikeFooter(t, h, keys, "")
				if h.leader || spikeInput(h) != input || !reflect.DeepEqual(native, ghosttySnapshot(t, h.terminal)) {
					t.Fatal("locking with Escape/Ctrl+P changed native input or the caret")
				}
			}
			frontendChord(h, 'v')
			if h.leader || h.placement != placementLeft {
				t.Fatal("one unlocked layout action did not relock")
			}
			assertSpikeFooter(t, h, keys, "Placement: left")
			spikeKey(h, 'm', 0)
			awaitSpike(t, h, "next bare key returns to the child", func() bool { return spikeInput(h) == input+"m" })
			assertSpikeFooter(t, h, keys, "")
			h.notice = "LOCK_NOTICE"
			assertSpikeFooter(t, h, keys, "LOCK_NOTICE")
		})
	}
}

func TestHostStarshipKeepsNativeWideInputAcrossPaneResize(t *testing.T) {
	frontendTools(t)
	config := filepath.Join(t.TempDir(), "starship.toml")
	if err := os.WriteFile(config, []byte(smokeStarshipConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("STARSHIP_CONFIG", config)
	h, err := newHost(120, 16, smokeEnvironment(t.TempDir()), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.close(); err != nil {
			t.Error(err)
		}
	})
	awaitSpike(t, h, "native Starship prompt", func() bool { return h.prompt.ready && spikeInput(h) == "" })
	input := "echo parked-界é🙂"
	visible := func() bool {
		return h.prompt.ready && spikeInput(h) == input && strings.Contains(spikeFrameText(h.frame), "界❯ "+input)
	}
	h.Update(tea.PasteMsg{Content: input})
	awaitSpike(t, h, "native wide Starship input", visible)
	frontendChord(h, 'x')
	awaitFrontendReady(t, h, "MENU")
	p := h.pane
	spikeKey(h, ']', tea.ModCtrl)
	frontendChord(h, tea.KeyTab)
	input += "-dock"
	h.Update(tea.PasteMsg{Content: "-dock"})
	awaitSpike(t, h, "docked native wide input remains editable", visible)
	frontendChord(h, 't')
	input += "-hide"
	h.Update(tea.PasteMsg{Content: "-hide"})
	awaitSpike(t, h, "hidden pane restores native wide prompt and input", visible)
	frontendChord(h, 'x')
	if h.pane != p || !h.paneFocused() {
		t.Fatal("show replaced the pane or lost focus")
	}
	awaitSpike(t, h, "show preserves native wide Starship input after SIGWINCH", visible)
	frontendChord(h, 'c')
	awaitSpike(t, h, "close keeps native wide Starship input", visible)
}

func TestHostRealPTYResizeExitAndCleanup(t *testing.T) {
	h := newSpikeTestHost(t)
	rc := h.child.rc
	h.Update(tea.WindowSizeMsg{Width: 35, Height: 6})
	h.Update(tea.PasteMsg{Content: "stty size"})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "PTY geometry excludes the footer", func() bool {
		return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), "5 35")
	})
	h.Update(tea.WindowSizeMsg{Width: 5, Height: 1})
	if h.frame.Cols != 5 || h.frame.Rows != 1 {
		t.Fatalf("tiny child geometry = %dx%d", h.frame.Cols, h.frame.Rows)
	}
	h.Update(tea.WindowSizeMsg{Width: 84, Height: 12})
	h.Update(tea.PasteMsg{Content: "exit 37"})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "child exit status", func() bool { return h.code == 37 })
	start := time.Now()
	if err := h.close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*closeGrace {
		t.Fatal("PTY cleanup was not bounded")
	}
	if _, err := os.Stat(rc); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("private rc survived cleanup: %v", err)
	}
	if err := h.close(); err != nil {
		t.Fatal("idempotent close:", err)
	}
	if message := h.child.nextOutput(); message != nil {
		t.Fatalf("output command survived close: %T", message)
	}
}

func TestHostBoundsShutdownWithNoisyStubbornBash(t *testing.T) {
	h := newSpikeTestHost(t)
	h.Update(tea.PasteMsg{Content: "trap '' HUP; while :; do printf 'NOISY_CHILD\\n'; done"})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "noisy child that ignores HUP", func() bool {
		return h.prompt.phase == "running" && strings.Contains(spikeFrameText(h.frame), "NOISY_CHILD")
	})
	start := time.Now()
	if err := h.close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*closeGrace {
		t.Fatal("noisy stubborn child escaped the shutdown bound")
	}
	select {
	case <-h.child.exited:
	default:
		t.Fatal("Bash was not reaped")
	}
	select {
	case <-h.child.readerDone:
	default:
		t.Fatal("PTY reader survived cleanup")
	}
}

func TestHostRoutesFocusMouseAndShowsTerminalErrors(t *testing.T) {
	h := newSpikeTestHost(t)
	command := `printf '\e[?1004h\e[?1003h\e[?1006h'; saved=$(stty -g); stty raw -echo; printf 'ROUTING_READY\r\n'; IFS= read -r -N 3 focus; IFS= read -r -N 9 mouse; stty "$saved"; printf '\e[?1004l\e[?1003l\e[?1006l\nROUTE:%q:%q\n' "$focus" "$mouse"`
	h.Update(tea.PasteMsg{Content: command})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "child mouse/focus modes", func() bool {
		return h.frame.MouseTracking && strings.Contains(spikeFrameText(h.frame), "ROUTING_READY") && h.prompt.phase == "running"
	})
	h.Update(tea.BlurMsg{})
	h.Update(tea.MouseClickMsg{X: 1, Y: h.rows - 1, Button: tea.MouseLeft})
	h.Update(tea.MouseClickMsg{X: 1, Y: 1, Button: tea.MouseLeft})
	awaitSpike(t, h, "focus and shell mouse bytes (not footer mouse)", func() bool {
		return h.prompt.ready && strings.Contains(spikeFrameText(h.frame), `ROUTE:$'\E[O':$'\E[<0;2;2M'`)
	})
	if h.View().MouseMode != tea.MouseModeNone {
		t.Fatal("host retained mouse capture after the child disabled it")
	}
	h.terminal.Close()
	spikeKey(h, tea.KeyEnter, 0)
	if h.err == nil || !strings.Contains(ansi.Strip(h.View().Content), "error: libghostty-vt input:") {
		t.Fatalf("terminal failure was hidden: err=%v view=%q", h.err, h.View().Content)
	}
}
