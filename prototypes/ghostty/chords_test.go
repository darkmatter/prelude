package main

import (
	"errors"
	"flag"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// These temporary executable fixtures exercise the real pane PTY/engine through
// Update without depending on the parent's Nix wrapper/configuration. Their
// bounded hex display lets assertions distinguish routing from echoed input.
func frontendTools(t *testing.T) string {
	t.Helper()
	// This x is a PTY routing fixture, not the generated catalogue dispatcher.
	t.Setenv("PRELUDE_COMPLETION_INIT", "")
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for command, label := range map[string]string{"x": "MENU", "docs": "DOCS"} {
		content := fmt.Sprintf("#!%s\nexport GHOSTTY_FRONTEND_TOOL=%s\nexec %q -test.run=^TestFrontendToolHelper$ -- \"$@\"\n", bash, label, binary)
		if err := os.WriteFile(filepath.Join(dir, command), []byte(content), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeMotdFixture(t, dir, fixtureMotd)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func TestFrontendToolHelper(t *testing.T) {
	label := os.Getenv("GHOSTTY_FRONTEND_TOOL")
	if label == "" {
		return
	}
	args := flag.Args()
	var selectionOutput string
	// This fixture also runs directly in Bash, where x is argument-free.
	if label == "MENU" {
		if len(args) != 0 {
			selectionOutput = menuSelectionOutput(t, args)
		}
	} else if len(args) != 0 {
		t.Fatalf("docs args = %q, want no arguments", args)
	}
	if _, err := term.MakeRaw(int(os.Stdin.Fd())); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("\x1b[?1004h\x1b[?1003h\x1b[?1006h%s_READY\r\nINPUT:", label)
	input := make([]byte, 0, 64)
	for {
		var data [1]byte
		if _, err := os.Stdin.Read(data[:]); err != nil {
			os.Exit(0)
		}
		if data[0] == 'q' {
			os.Exit(7)
		}
		if source := os.Getenv("GHOSTTY_FRONTEND_SELECTION"); data[0] == '\r' && selectionOutput != "" && source != "" {
			if err := os.WriteFile(selectionOutput, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			os.Exit(0)
		}
		input = append(input, data[0])
		if len(input) > 64 {
			input = input[len(input)-64:]
		}
		fmt.Printf("\x1b[2;1H\x1b[2KINPUT:%x", input)
	}
}

func frontendPTYSize(t *testing.T, process *shellProcess) [2]int {
	t.Helper()
	raw, err := process.ptmx.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var size *unix.Winsize
	var ioctlErr error
	if err := raw.Control(func(fd uintptr) {
		size, ioctlErr = unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	}); errors.Join(err, ioctlErr) != nil {
		t.Fatal(errors.Join(err, ioctlErr))
	}
	return [2]int{int(size.Col), int(size.Row)}
}

func frontendCompactText(frame terminalFrame) string {
	return strings.Join(strings.Fields(spikeFrameText(frame)), "")
}

func frontendChord(h *host, code rune) tea.Cmd {
	h.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	_, cmd := h.Update(tea.KeyPressMsg{Code: code})
	return cmd
}

// The same UI loop receives both children's events; no goroutine touches native
// state. Like the existing host fixtures, tests consume process channels directly
// instead of launching Bubble Tea's renderer/commands.
func awaitFrontend(t *testing.T, h *host, what string, check func() bool) {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		if h.err != nil {
			t.Fatalf("%s: host failed: %v", what, h.err)
		}
		if check() {
			return
		}
		p := h.pane
		var output <-chan outputMsg
		var exited <-chan struct{}
		if h.currentPane(p) {
			output, exited = p.process.output, p.process.exited
		}
		select {
		case msg := <-h.child.output:
			h.Update(msg)
		case msg := <-output:
			h.Update(paneOutputMsg{pane: p, output: msg})
		case <-exited:
			// Honor the production final-output drain rather than racing the
			// PTY reader just because the wait result is already available.
			if msg := p.nextOutput(); msg != nil {
				h.Update(msg)
			}
		case <-h.child.exited:
			t.Fatalf("main Bash unexpectedly exited during %s: %v", what, h.child.waitErr)
		case <-timer.C:
			paneText := "(no pane)"
			if p != nil {
				paneText = spikeFrameText(p.frame)
			}
			t.Fatalf("waiting for %s; footer=%q\nshell:\n%s\npane:\n%s", what, h.footer(), spikeFrameText(h.frame), paneText)
		}
	}
}

func awaitFrontendReady(t *testing.T, h *host, label string) {
	t.Helper()
	awaitFrontend(t, h, label+" pane ready", func() bool {
		return h.pane != nil && strings.Contains(spikeFrameText(h.pane.frame), label+"_READY")
	})
}

func TestFrontendLeaderConsumesOnlyPrefixedChords(t *testing.T) {
	h := newSpikeTestHost(t)
	if !h.terminalFocused || h.placement != placementFloating || h.paneVisible || h.pane != nil {
		t.Fatal("new host must start outer-focused with no visible pane in floating placement")
	}
	for _, code := range "xd?" {
		spikeKey(h, code, 0)
	}
	awaitSpike(t, h, "unprefixed surface keys in readline", func() bool { return spikeInput(h) == "xd?" })
	spikeKey(h, 'p', tea.ModCtrl)
	if !h.leader || !strings.Contains(ansi.Strip(h.View().Content), "UNLOCKED    m  motd   x  menu") {
		t.Fatal("prefix was not consumed or its hints were hidden")
	}
	spikeKey(h, 'z', 0)
	if h.leader || !strings.Contains(h.notice, "Unknown Ctrl+P chord") || h.surface != surfaceNone {
		t.Fatal("invalid chord must be consumed with useful guidance")
	}
	h.Update(tea.PasteMsg{Content: "-safe"})
	awaitSpike(t, h, "invalid chord not injected", func() bool { return spikeInput(h) == "xd?-safe" })
	spikeKey(h, 'p', tea.ModCtrl)
	spikeKey(h, tea.KeyEscape, 0)
	if h.leader || spikeInput(h) != "xd?-safe" {
		t.Fatal("Escape should cancel only the prefix")
	}
	spikeKey(h, 'p', tea.ModCtrl)
	h.Update(tea.PasteMsg{Content: "-paste"})
	awaitSpike(t, h, "paste cancels pending prefix", func() bool { return spikeInput(h) == "xd?-safe-paste" })
	if h.leader {
		t.Fatal("paste left a pending prefix")
	}
	spikeKey(h, 'p', tea.ModCtrl)
	h.Update(tea.BlurMsg{})
	if h.leader || h.terminalFocused || h.View().Cursor != nil {
		t.Fatal("blur must cancel the prefix and hide the active cursor")
	}
	h.Update(tea.FocusMsg{})
	spikeKey(h, 'v', tea.ModCtrl)
	spikeKey(h, 'p', tea.ModCtrl)
	awaitSpike(t, h, "quoted Ctrl+P reaching readline", func() bool { return strings.Contains(spikeInput(h), "^P") })
	if h.leader {
		t.Fatal("quoted Ctrl+P incorrectly entered the leader")
	}
	spikeKey(h, 'c', tea.ModCtrl)
	awaitSpike(t, h, "normal Ctrl+C reaches Bash", func() bool { return h.prompt.ready && spikeInput(h) == "" })
	spikeShellStatus(t, h, 130)
	for _, want := range []placement{placementLeft, placementRight, placementTop, placementBottom, placementFloating} {
		frontendChord(h, 'v')
		if h.placement != want || h.pane != nil || !strings.Contains(h.notice, want.String()) {
			t.Fatalf("closed-pane placement cycle: got=%s want=%s notice=%q", h.placement, want, h.notice)
		}
	}
	for _, step := range []struct {
		key  rune
		want placement
	}{{'[', placementBottom}, {'\\', placementTop}, {']', placementBottom}, {']', placementFloating}} {
		spikeKey(h, 'p', tea.ModCtrl)
		spikeKey(h, step.key, tea.ModCtrl)
		if h.leader || h.placement != step.want || h.pane != nil || spikeInput(h) != "" {
			t.Fatalf("direct layout key must cancel the prefix without reaching Bash: key=%q placement=%s leader=%t", step.key, h.placement, h.leader)
		}
	}
}

func TestFrontendAltActionsAndOneShotLocking(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		upper, shifted, withText bool
	}{
		{"Lowercase", false, false, false},
		{"UppercaseCode", true, false, false},
		{"ShiftedCode", false, true, false},
		{"ShiftedText", false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frontendTools(t)
			h := newSpikeTestHost(t)
			alt := func(code rune) {
				upper := code - ('a' - 'A')
				key := tea.KeyPressMsg{Code: code, Mod: tea.ModAlt}
				if tc.upper {
					key.Code = upper
				}
				if tc.shifted {
					key.Mod |= tea.ModShift
					key.ShiftedCode = upper
				}
				if tc.withText {
					key.Text = string(upper)
				}
				h.Update(key)
			}
			for _, code := range "mxd" {
				spikeKey(h, code, 0)
			}
			awaitSpike(t, h, "bare m/x/d reach real Bash readline", func() bool { return spikeInput(h) == "mxd" })
			if h.pane != nil || h.surface != surfaceNone {
				t.Fatal("bare m/x/d opened a host surface")
			}
			native := ghosttySnapshot(t, h.terminal)
			spikeKey(h, 'p', tea.ModCtrl)
			assertChromeFooter(t, h, spikeLeaderKeys[:7], "", nil)
			spikeKey(h, 'p', tea.ModCtrl)
			assertChromeFooter(t, h, spikeNavigationKeys, "", nil)
			if h.leader || !reflect.DeepEqual(native, ghosttySnapshot(t, h.terminal)) {
				t.Fatal("double Ctrl+P forwarded a literal key instead of locking")
			}
			spikeKey(h, 'p', tea.ModCtrl)
			alt('x')
			awaitFrontendReady(t, h, "MENU")
			p := h.pane
			if h.leader || !h.paneFocused() || !h.paneVisible || h.surface != surfaceMenu || spikeInput(h) != "mxd" {
				t.Fatal("direct Alt+x failed to open/focus the menu and relock without changing Bash input")
			}
			assertChromeFooter(t, h, spikeNavigationKeys, "menu | floating | focus:pane | running", h.palette.Success)
			for _, code := range "mxd" {
				spikeKey(h, code, 0)
			}
			for _, code := range "mxd" {
				spikeKey(h, 'v', tea.ModCtrl)
				spikeKey(h, code, tea.ModAlt)
			}
			awaitFrontend(t, h, "bare keys and quoted Alt+m/x/d reach the focused child", func() bool {
				return strings.Contains(frontendCompactText(p.frame), fmt.Sprintf("INPUT:%x", "mxd\x16\x1bm\x16\x1bx\x16\x1bd")) && spikeInput(h) == "mxd"
			})
			if h.pane != p || !h.paneFocused() || h.leader {
				t.Fatal("Ctrl+V failed to escape the direct Alt binding")
			}
			frontendChord(h, 't')
			if h.leader || h.pane != p || h.paneVisible || h.focusPane || p.closed ||
				frontendPTYSize(t, h.child) != [2]int{84, 11} || !reflect.DeepEqual(native, ghosttySnapshot(t, h.terminal)) {
				t.Fatal("one-shot t failed to hide the same pane while preserving native input/PTY/caret")
			}
			frontendChord(h, 'x')
			if h.leader || h.pane != p || !h.paneFocused() {
				t.Fatal("one-shot x did not restore the hidden menu and relock")
			}
			alt('d')
			awaitFrontendReady(t, h, "DOCS")
			docs := h.pane
			if h.surface != surfaceDocs || !h.paneFocused() || docs == p || !p.closed || h.leader {
				t.Fatal("direct Alt+d did not replace the menu with focused docs in locked mode")
			}
			paneSize := frontendPTYSize(t, docs.process)
			alt('m')
			awaitFrontend(t, h, "direct Alt+m runs MOTD in Bash and restores parked input", func() bool {
				return h.prompt.ready && spikeInput(h) == "mxd" && strings.Count(spikeFrameText(h.frame), "MOTD_SHELL:") == 2
			})
			if h.leader || h.pane != docs || h.paneVisible || h.focusPane || docs.closed ||
				frontendPTYSize(t, docs.process) != paneSize || frontendPTYSize(t, h.child) != [2]int{84, 11} ||
				strings.Contains(spikeFrameText(docs.frame), "MOTD_SHELL:") {
				t.Fatal("Alt+m destroyed/resized the hidden pane or sent MOTD to it instead of Bash")
			}
			assertSpikeFooter(t, h, spikeFooterKeyText(spikeNavigationKeys), "docs hidden | running")
		})
	}
}

func TestFrontendPaneInputLifecycleAndStaleEvents(t *testing.T) {
	frontendTools(t)
	h := newSpikeTestHost(t)
	frontendChord(h, 'x')
	awaitFrontendReady(t, h, "MENU")
	p := h.pane
	if h.surface != surfaceMenu || !h.paneVisible || !h.paneFocused() || p.done || h.leader || h.placement != placementFloating {
		t.Fatal("one-shot x must open the live focused floating menu and relock")
	}
	for _, code := range "md?x" {
		spikeKey(h, code, 0)
	}
	spikeKey(h, 'c', tea.ModCtrl)
	spikeKey(h, 'p', tea.ModCtrl)
	spikeKey(h, 'p', tea.ModCtrl)
	spikeKey(h, 'v', tea.ModCtrl)
	spikeKey(h, 'p', tea.ModCtrl)
	spikeKey(h, 'v', tea.ModCtrl)
	spikeKey(h, 'g', tea.ModCtrl)
	spikeKey(h, 'g', tea.ModCtrl)
	spikeKey(h, 'p', tea.ModCtrl)
	spikeKey(h, 'g', tea.ModCtrl)
	const received = "md?x\x03\x16\x10\x16\x07\x07\x07"
	awaitFrontend(t, h, "focused child receives text, Ctrl+C, quoted controls, and Ctrl+G even after unlock", func() bool {
		return strings.Contains(spikeFrameText(p.frame), fmt.Sprintf("INPUT:%x", received))
	})
	if h.surface != surfaceMenu || !h.paneVisible || !h.paneFocused() || h.leader || spikeInput(h) != "" {
		t.Fatal("unprefixed/quoted pane input changed host state or leaked into Bash")
	}
	spikeKey(h, 'p', tea.ModCtrl)
	h.Update(tea.PasteMsg{Content: "A"})
	awaitFrontend(t, h, "paste cancels leader and reaches focused pane", func() bool {
		return strings.Contains(spikeFrameText(p.frame), fmt.Sprintf("INPUT:%x", received+"A"))
	})
	if h.leader || spikeInput(h) != "" {
		t.Fatal("pane paste left the leader pending or leaked into Bash")
	}
	before := p.frame
	// Leave genuine child output queued across the hide, not a synthetic image.
	spikeKey(h, 'H', 0)
	frontendChord(h, 't')
	if h.paneVisible || h.focusPane || h.leader || h.pane != p || !h.currentPane(p) || !reflect.DeepEqual(before, p.frame) ||
		h.View().MouseMode != tea.MouseModeNone {
		t.Fatal("one-shot t must retain the live image/reader and return input/capture to Bash")
	}
	assertSpikeFooter(t, h, spikeFooterKeyText(spikeHiddenKeys), "menu hidden | running")
	awaitFrontend(t, h, "hidden pane continues absorbing child output", func() bool {
		return strings.Contains(frontendCompactText(p.frame), fmt.Sprintf("INPUT:%x", received+"AH\x1b[O"))
	})
	h.Update(tea.PasteMsg{Content: "echo parked"})
	awaitFrontend(t, h, "hidden pane sends paste to Bash", func() bool { return spikeInput(h) == "echo parked" })
	frontendChord(h, 't')
	spikeKey(h, 'a', 0)
	awaitFrontend(t, h, "show restores input and navigation state in the same focused pane", func() bool {
		return strings.Contains(frontendCompactText(p.frame), fmt.Sprintf("INPUT:%x", received+"AH\x1b[O\x1b[Ia"))
	})
	if h.pane != p || !h.paneVisible || !h.paneFocused() || spikeInput(h) != "echo parked" {
		t.Fatal("show replaced the pane or changed parked Bash input")
	}
	frontendChord(h, tea.KeyTab)
	h.Update(tea.PasteMsg{Content: "-tab"})
	awaitFrontend(t, h, "focus toggle still sends paste to Bash", func() bool { return spikeInput(h) == "echo parked-tab" })
	frontendChord(h, tea.KeyTab)
	spikeKey(h, 'q', 0)
	awaitFrontend(t, h, "surface exit retained without quitting main shell", func() bool { return p.done })
	if h.pane != p || p.closed || p.terminal.native == nil || p.exitCode != 7 || h.focusPane || h.paneError != nil || h.code != 0 {
		t.Fatalf("finished pane state: pane=%+v focus=%v error=%v shellCode=%d", p, h.focusPane, h.paneError, h.code)
	}
	if !strings.Contains(ansi.Strip(h.View().Content), "exited:7") {
		t.Fatal("retained pane's exit status is not visible")
	}
	final := p.frame
	frontendChord(h, 't')
	if h.paneVisible {
		t.Fatal("finished pane did not hide with its retained exit status")
	}
	assertSpikeFooter(t, h, spikeFooterKeyText(spikeHiddenKeys), "menu hidden | exited:7")
	frontendChord(h, 't')
	if h.pane != p || !h.paneVisible || h.focusPane || p.closed || !reflect.DeepEqual(final, p.frame) {
		t.Fatal("one-shot t restarted or focused a finished pane instead of showing its final image")
	}
	frontendChord(h, 'x')
	if h.pane == nil || h.pane == p || !p.closed || !h.paneFocused() {
		t.Fatal("same finished surface must restart, not toggle closed")
	}
	awaitFrontendReady(t, h, "MENU")
	current := h.pane
	before = current.frame
	for _, msg := range []tea.Msg{
		paneOutputMsg{pane: p, output: outputMsg{data: []byte("STALE_IMAGE")}},
		paneExitMsg{pane: p, err: errors.New("stale exit")},
		paneFailureMsg{pane: p, err: errors.New("stale failure")},
	} {
		if _, cmd := h.Update(msg); cmd != nil {
			t.Fatalf("stale %T returned a command", msg)
		}
	}
	if h.pane != current || h.paneError != nil || !reflect.DeepEqual(current.frame, before) {
		t.Fatal("stale messages affected the replacement pane")
	}
	frontendChord(h, 'x')
	if h.pane != current || h.paneVisible || current.closed || !h.currentPane(current) {
		t.Fatal("same running surface must hide without destroying the pane")
	}
	frontendChord(h, 'c')
	if h.pane != nil || h.surface != surfaceNone || h.paneVisible || !current.closed || h.focusPane || current.terminal.native != nil {
		t.Fatal("close chord must destroy even a hidden pane")
	}
	paneTestProcessClosed(t, current)
	if _, cmd := h.Update(paneFailureMsg{pane: current, err: errors.New("late failure")}); cmd != nil || h.err != nil || h.paneError != nil {
		t.Fatal("closed pane failure escaped into main-shell state")
	}
	if h.footer() != baseFooter {
		t.Fatal("closing a pane did not restore the exact base footer")
	}
}

func TestFrontendSameSurfaceChordsRetainHiddenPanes(t *testing.T) {
	frontendTools(t)
	h := newSpikeTestHost(t)
	spikeKey(h, ']', tea.ModCtrl) // A split must expand Bash when hidden.
	var previous *pane
	for _, surface := range []struct {
		key   rune
		kind  surfaceKind
		label string
	}{{'x', surfaceMenu, "MENU"}, {'d', surfaceDocs, "DOCS"}} {
		frontendChord(h, surface.key)
		awaitFrontendReady(t, h, surface.label)
		if previous != nil {
			if !previous.closed || previous.terminal.native != nil {
				t.Fatal("switching surface kind retained the previous child/engine")
			}
			paneTestProcessClosed(t, previous)
		}
		p := h.pane
		process, engine, pid := p.process, p.terminal, p.process.cmd.Process.Pid
		ptmx, native, reader := process.ptmx, engine.native, process.readerDone
		spikeKey(h, 'a', 0)
		awaitFrontend(t, h, "pane input before hiding", func() bool { return strings.Contains(spikeFrameText(p.frame), "INPUT:61") })
		before := p.frame
		frontendChord(h, surface.key)
		if h.surface != surface.kind || h.pane != p || h.paneVisible || h.focusPane || !h.currentPane(p) ||
			p.process != process || p.terminal != engine || engine.native != native || process.ptmx != ptmx || process.readerDone != reader || p.process.cmd.Process.Pid != pid ||
			!reflect.DeepEqual(before, p.frame) || !h.layout().Body.Empty() || h.frame.Cols != h.cols || h.frame.Rows != shellRows(h.rows) ||
			frontendPTYSize(t, h.child) != [2]int{h.cols, shellRows(h.rows)} {
			t.Fatalf("same %s chord did not retain the pane and give Bash the full usable area", surface.kind)
		}
		assertSpikeFooter(t, h, spikeFooterKeyText(spikeHiddenKeys), surface.kind.String()+" hidden | running")
		for _, done := range []<-chan struct{}{process.readerDone, process.exited, process.stop} {
			select {
			case <-done:
				t.Fatal("hiding stopped a pane reader, process, or output command")
			default:
			}
		}
		h.Update(tea.PasteMsg{Content: "stty size"})
		awaitFrontend(t, h, "hidden pane routes input to Bash", func() bool { return spikeInput(h) == "stty size" })
		spikeKey(h, tea.KeyEnter, 0)
		awaitFrontend(t, h, "Bash runs at full size while pane is hidden", func() bool {
			return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), "11 84")
		})
		hidden := p.frame
		h.Update(tea.WindowSizeMsg{Width: 96, Height: 18})
		h.Update(tea.WindowSizeMsg{Width: 84, Height: 12})
		if h.paneVisible || !h.layout().Body.Empty() || !reflect.DeepEqual(hidden, p.frame) {
			t.Fatal("resize restored an explicitly hidden pane or changed its saved geometry")
		}
		frontendChord(h, surface.key)
		spikeKey(h, 'b', 0)
		awaitFrontend(t, h, "same surface restores input state and focus", func() bool {
			return strings.Contains(frontendCompactText(p.frame), "INPUT:611b5b4f1b5b4962")
		})
		if h.pane != p || !h.paneVisible || !h.paneFocused() || p.process != process || p.terminal != engine ||
			engine.native != native || process.ptmx != ptmx || process.readerDone != reader || p.process.cmd.Process.Pid != pid {
			t.Fatalf("showing %s restarted the child or failed to focus it", surface.kind)
		}
		previous = p
	}
}

func TestFrontendPlacementResizesWithoutReplacingOrDiscardingPane(t *testing.T) {
	dir := frontendTools(t)
	h := newSpikeTestHost(t)
	h.Update(tea.PasteMsg{Content: fmt.Sprintf("%q", filepath.Join(dir, "x"))})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "main raw reader ready before resizing", func() bool {
		return h.prompt.phase == "running" && strings.Contains(spikeFrameText(h.frame), "MENU_READY")
	})
	frontendChord(h, 'd')
	awaitFrontendReady(t, h, "DOCS")
	p, process, engine := h.pane, h.pane.process, h.pane.terminal

	for _, direction := range []struct {
		key        rune
		placements []placement
	}{
		{']', []placement{placementLeft, placementRight, placementTop, placementBottom, placementFloating}},
		{'[', []placement{placementBottom, placementTop, placementRight, placementLeft, placementFloating}},
	} {
		for _, want := range direction.placements {
			spikeKey(h, direction.key, tea.ModCtrl)
			layout := h.layout()
			if h.placement != want || h.pane != p || p.process != process || p.terminal != engine || p.done || !h.paneFocused() {
				t.Fatal("placement change replaced/finished the live pane or changed focus")
			}
			if p.frame.Cols != layout.Body.Dx() || p.frame.Rows != layout.Body.Dy() || h.frame.Cols != layout.Shell.Dx() || h.frame.Rows != layout.Shell.Dy() {
				t.Fatalf("engines did not follow %s geometry: layout=%+v shell=%dx%d pane=%dx%d", want, layout, h.frame.Cols, h.frame.Rows, p.frame.Cols, p.frame.Rows)
			}
			if frontendPTYSize(t, process) != [2]int{layout.Body.Dx(), layout.Body.Dy()} ||
				frontendPTYSize(t, h.child) != [2]int{layout.Shell.Dx(), layout.Shell.Dy()} {
				t.Fatal("placement changed engine geometry without resizing both live PTYs")
			}
		}
	}
	before := p.frame
	h.Update(tea.WindowSizeMsg{Width: 2, Height: 2})
	if !h.layout().Body.Empty() || !h.paneVisible || h.focusPane || h.pane != p || p.process != process || !reflect.DeepEqual(before, p.frame) ||
		h.footer() != "" {
		t.Fatal("tiny-size suspension must retain visibility and the last frame without painting unreadable footer content")
	}
	spikeKey(h, 's', 0)
	h.Update(tea.WindowSizeMsg{Width: 96, Height: 18})
	// Refresh the raw reader's bounded input display after growing; its old
	// image was intentionally off-screen at two columns, like readline's PS1.
	spikeKey(h, 'b', 0)
	awaitFrontend(t, h, "hidden pane input goes to shell", func() bool {
		return strings.Contains(frontendCompactText(h.frame), "7362")
	})
	layout := h.layout()
	if h.pane != p || !h.paneVisible || h.focusPane || p.process != process || p.done || p.frame.Cols != layout.Body.Dx() || p.frame.Rows != layout.Body.Dy() {
		t.Fatal("growing workspace did not restore the same pane geometry without stealing focus")
	}
	frontendChord(h, tea.KeyTab)
	spikeKey(h, 'a', 0)
	awaitFrontend(t, h, "grown pane resumes input after focus loss/gain", func() bool {
		return strings.Contains(frontendCompactText(p.frame), "INPUT:1b5b4f1b5b4961")
	})
}

func TestFrontendMissingToolAndPaneFailureDoNotQuitShell(t *testing.T) {
	frontendTools(t)
	h := newSpikeTestHost(t)
	path := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	h.env = os.Environ()
	frontendChord(h, 'x')
	if h.err != nil || h.surface != surfaceMenu || h.pane != nil || h.paneError == nil || h.focusPane {
		t.Fatalf("unsafe startup failure: shellErr=%v paneErr=%v pane=%v", h.err, h.paneError, h.pane)
	}
	view := ansi.Strip(h.View().Content)
	if !strings.Contains(view, "error:") || !strings.Contains(view, "pane command") {
		t.Fatalf("missing-tool error is not visible: %q", view)
	}
	startupError := h.paneError
	frontendChord(h, 't')
	if h.paneVisible || h.pane != nil || h.paneError != startupError {
		t.Fatal("one-shot t did not hide the startup error without retrying")
	}
	assertSpikeFooter(t, h, spikeFooterKeyText(spikeHiddenKeys), "menu hidden | error")
	frontendChord(h, 't')
	if !h.paneVisible || h.pane != nil || h.paneError != startupError || h.focusPane {
		t.Fatal("one-shot t retried or focused a failed startup instead of showing its error")
	}
	h.Update(tea.PasteMsg{Content: "safe-shell"})
	awaitSpike(t, h, "Bash remains usable after missing tool", func() bool { return spikeInput(h) == "safe-shell" })
	t.Setenv("PATH", path)
	h.env = os.Environ()
	frontendChord(h, 'x')
	awaitFrontendReady(t, h, "MENU")
	p := h.pane
	before := p.frame
	h.Update(paneFailureMsg{pane: p, err: errors.New("fixture pane failure")})
	if h.err != nil || !p.done || p.closed || h.focusPane || h.paneError == nil || !reflect.DeepEqual(before, p.frame) {
		t.Fatal("pane failure must retain its image, report error, and return focus without quitting Bash")
	}
	if !strings.Contains(ansi.Strip(h.View().Content), "fixture pane failure") {
		t.Fatal("live pane failure is not visible in the notice")
	}
	frontendChord(h, tea.KeyEscape)
	if h.pane != p || h.surface != surfaceMenu {
		t.Fatal("Escape cancelled the pane instead of just the prefix")
	}
	failure := h.paneError
	frontendChord(h, 't')
	if h.paneVisible {
		t.Fatal("failed pane did not hide with its retained error status")
	}
	assertSpikeFooter(t, h, spikeFooterKeyText(spikeHiddenKeys), "menu hidden | error")
	frontendChord(h, 't')
	if h.pane != p || !h.paneVisible || h.focusPane || p.closed || h.paneError != failure || !reflect.DeepEqual(before, p.frame) {
		t.Fatal("one-shot t restarted, focused, or discarded a failed pane")
	}
	frontendChord(h, 'x')
	awaitFrontendReady(t, h, "MENU")
	if h.pane == p || !p.closed || h.paneError != nil || !h.paneFocused() {
		t.Fatal("explicit same-surface chord did not restart the failed pane")
	}
	paneTestProcessClosed(t, p)
	frontendChord(h, 'c')
	if h.pane != nil || h.paneError != nil || h.footer() != baseFooter {
		t.Fatal("close chord failed to clear the error pane")
	}
}

func TestFrontendMotdAndQuestionMarkRemainShellOnly(t *testing.T) {
	frontendTools(t)
	h := newSpikeTestHost(t)
	for _, key := range []tea.KeyPressMsg{{Code: '?'}, {Code: '/', ShiftedCode: '?', Mod: tea.ModShift, Text: "?"}} {
		spikeKey(h, 'p', tea.ModCtrl)
		h.Update(key)
		if h.leader || h.pane != nil || h.paneVisible || h.surface != surfaceNone || !strings.Contains(h.notice, "Unknown Ctrl+P chord") {
			t.Fatal("Ctrl+P ? must be unsupported, never a MOTD pane")
		}
	}
	spikeKey(h, '?', 0)
	awaitSpike(t, h, "bare question mark reaches readline", func() bool { return spikeInput(h) == "?" })
	spikeKey(h, 'u', tea.ModCtrl)
	h.Update(tea.PasteMsg{Content: "motd"})
	awaitSpike(t, h, "MOTD is an ordinary Bash command", func() bool { return spikeInput(h) == "motd" })
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "MOTD output stays in Bash", func() bool {
		return h.prompt.ready && spikeInput(h) == "" &&
			strings.Count(spikeFrameText(h.frame), "MOTD_SHELL:") == 2
	})
	if h.pane != nil || h.surface != surfaceNone || h.footer() != baseFooter {
		t.Fatal("shell MOTD changed the host surface or footer")
	}
}

func TestFrontendFocusReportingMouseTargetsAndOffsets(t *testing.T) {
	dir := frontendTools(t)
	h := newSpikeTestHost(t)
	// Run a raw fixture as Bash's foreground program so both children advertise
	// focus/mouse reporting. Opening docs must not inject any command into Bash.
	h.Update(tea.PasteMsg{Content: fmt.Sprintf("%q", filepath.Join(dir, "x"))})
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "main raw reader ready", func() bool {
		return h.prompt.phase == "running" && h.frame.MouseTracking && strings.Contains(spikeFrameText(h.frame), "MENU_READY")
	})
	spikeKey(h, 'g', tea.ModCtrl)
	spikeKey(h, 'p', tea.ModCtrl)
	spikeKey(h, 'g', tea.ModCtrl)
	awaitSpike(t, h, "Ctrl+G remains foreground-owned both locked and unlocked", func() bool {
		return !h.leader && h.pane == nil && strings.Contains(spikeFrameText(h.frame), "INPUT:0707")
	})
	frontendChord(h, 'd')
	awaitFrontendReady(t, h, "DOCS")
	p := h.pane
	h.Update(tea.FocusMsg{})
	awaitFrontend(t, h, "only pane receives focus gain", func() bool {
		return strings.Contains(spikeFrameText(p.frame), "INPUT:1b5b49") && strings.Contains(spikeFrameText(h.frame), "INPUT:07071b5b4f")
	})
	if strings.Contains(spikeFrameText(h.frame), "1b5b49") {
		t.Fatal("inactive main shell received a focus gain")
	}
	h.Update(tea.BlurMsg{})
	awaitFrontend(t, h, "active pane receives outer blur", func() bool { return strings.Contains(spikeFrameText(p.frame), "INPUT:1b5b491b5b4f") })
	if h.View().Cursor != nil {
		t.Fatal("blurred outer terminal retained a cursor")
	}
	h.Update(tea.FocusMsg{})
	floating := h.layout()
	h.Update(tea.MouseClickMsg{X: floating.Panel.Min.X, Y: floating.Panel.Min.Y, Button: tea.MouseLeft})
	awaitFrontend(t, h, "former floating border cell routes to child coordinate 1,1", func() bool {
		return strings.Contains(frontendCompactText(p.frame), "1b5b3c303b313b314d")
	})
	frontendChord(h, 'v')
	layout := h.layout()
	frontendChord(h, tea.KeyTab)
	if h.paneFocused() || h.View().MouseMode != tea.MouseModeAllMotion {
		t.Fatal("mouse capture must include visible unfocused children")
	}
	h.Update(tea.MouseClickMsg{X: 0, Y: layout.FooterY, Button: tea.MouseLeft})
	if h.paneFocused() {
		t.Fatal("footer click changed focus")
	}
	h.Update(tea.MouseClickMsg{X: layout.Panel.Min.X, Y: layout.Panel.Min.Y, Button: tea.MouseLeft})
	if !h.paneFocused() {
		t.Fatal("former border cell did not focus the child")
	}
	h.Update(tea.MouseClickMsg{X: layout.Panel.Max.X - 1, Y: layout.Panel.Max.Y - 1, Button: tea.MouseLeft})
	awaitFrontend(t, h, "bottom-right former border cell is part of the child PTY", func() bool {
		want := fmt.Sprintf("%x", []byte(fmt.Sprintf("\x1b[<0;%d;%dM", layout.Body.Dx(), layout.Body.Dy())))
		return strings.Contains(frontendCompactText(p.frame), want)
	})
	h.Update(tea.MouseClickMsg{X: layout.Body.Min.X + 1, Y: layout.Body.Min.Y + 1, Button: tea.MouseLeft})
	awaitFrontend(t, h, "pane body mouse translated to local coordinates", func() bool {
		return strings.Contains(frontendCompactText(p.frame), "1b5b3c303b323b324d")
	})
	if !h.paneFocused() {
		t.Fatal("interactive body click did not focus the pane")
	}
	h.Update(tea.MouseClickMsg{X: layout.Shell.Min.X + 1, Y: layout.Shell.Min.Y + 1, Button: tea.MouseLeft})
	awaitFrontend(t, h, "offset shell mouse translated to local coordinates", func() bool {
		return strings.Contains(frontendCompactText(h.frame), "1b5b3c303b323b324d")
	})
	if h.paneFocused() {
		t.Fatal("shell click did not return keyboard focus")
	}
	frontendChord(h, 't')
	h.Update(tea.MouseClickMsg{X: layout.Panel.Min.X, Y: layout.Panel.Min.Y, Button: tea.MouseLeft})
	awaitFrontend(t, h, "hidden pane origin becomes full-size shell mouse coordinate 1,1", func() bool {
		return strings.Contains(frontendCompactText(h.frame), "1b5b3c303b313b314d")
	})
	if h.paneVisible || h.focusPane || h.pane != p {
		t.Fatal("click in a hidden pane's former body stole focus or discarded it")
	}
	frontendChord(h, 't')
	if !h.paneFocused() {
		t.Fatal("showing a healthy interactive pane did not restore focus")
	}
}

func TestFrontendCompletionAlignsDescriptionsWithoutBackground(t *testing.T) {
	h := &host{cols: 80, rows: 14, frame: spikeFrame(80, 13), terminalFocused: true,
		palette: chromePalette{
			Fg:      color.RGBA{20, 40, 60, 255},
			Bg:      color.RGBA{5, 10, 15, 255},
			Muted:   color.RGBA{35, 55, 75, 255},
			Accent:  color.RGBA{80, 100, 120, 255},
			Accent2: color.RGBA{140, 160, 180, 255},
		},
	}
	h.frame.CursorX, h.frame.CursorY = len(spikePrompt)+1, 1
	h.prompt = promptState{phase: "prompt", ready: true, anchor: 80 + len(spikePrompt), anchorCols: 80, promptRows: 1, promptRowsCols: 80}
	h.completion = &completionState{
		phase: completionPopup,
		snapshot: completionSnapshot{candidates: []completionCandidate{
			{name: "dev", description: "First description"},
			{name: "界:dev", description: "Second description"},
			{name: "long:command", description: "Third description"},
		}},
	}
	for i := range h.frame.Cells {
		h.frame.Cells[i] = uv.Cell{Content: " ", Width: 1, Style: uv.Style{Bg: color.RGBA{100, 30, 40, 255}}}
	}
	for x, char := range spikePrompt + "x" {
		h.frame.Cells[h.frame.Cols+x].Content = string(char)
	}
	layout := h.layout()
	area := h.completionArea(layout)
	if area.Dy() != 3 || area.Min.Y != h.frame.CursorY+1 || layout.FooterY != h.rows-1 ||
		layout.Shell.Dx() != h.frame.Cols || layout.Shell.Dy() != h.frame.Rows {
		t.Fatalf("candidate-only completion changed the prompt/native/footer geometry: %+v", layout)
	}
	view := h.View()
	lines := strings.Split(ansi.Strip(view.Content), "\n")
	if len(lines) != h.rows || strings.Count(ansi.Strip(view.Content), completionHint) != 1 ||
		strings.TrimSpace(lines[layout.FooterY]) != completionHint || strings.Contains(strings.Join(lines[:layout.FooterY], "\n"), completionHint) {
		t.Fatalf("completion hint must appear only in the single protected footer, not beneath candidates: %q", lines)
	}
	outer := vt.NewEmulator(h.cols, h.rows)
	defer outer.Close()
	if _, err := outer.Write([]byte(strings.ReplaceAll(h.View().Content, "\n", "\r\n"))); err != nil {
		t.Fatal(err)
	}
	column := area.Min.X + 2 + ansi.StringWidth("long:command") + 2
	for row, first := range []string{"F", "S", "T"} {
		fg := h.palette.Fg
		if row == 0 {
			fg = h.palette.Accent
		}
		if cell := outer.CellAt(column, area.Min.Y+row); cell == nil || cell.Content != first || cell.Style.Fg == nil ||
			color.RGBAModel.Convert(cell.Style.Fg) != color.RGBAModel.Convert(fg) {
			t.Fatalf("description on row %d lost alignment or its palette color at column %d: %+v", row, column, cell)
		}
	}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			if cell := outer.CellAt(x, y); cell != nil && cell.Style.Bg != nil {
				t.Fatalf("completion cell %d,%d imposed a background: %+v", x, y, cell)
			}
		}
	}
	if cell := outer.CellAt(area.Min.X, area.Min.Y); cell == nil || cell.Content != ">" || cell.Style.Attrs&uv.AttrBold == 0 ||
		cell.Style.Fg == nil || color.RGBAModel.Convert(cell.Style.Fg) != color.RGBAModel.Convert(h.palette.Accent) {
		t.Fatalf("selection must retain its marker, bold emphasis, and palette accent: %+v", cell)
	}
	assertChromeFooterCells(t, outer, h, spikeCompletionKeys, "", nil)
	if cell := outer.CellAt(area.Min.X, area.Max.Y); cell == nil || cell.Style.Bg == nil ||
		color.RGBAModel.Convert(cell.Style.Bg) != (color.RGBA{100, 30, 40, 255}) {
		t.Fatalf("completion painted a second footer over the native row below its candidates: %+v", cell)
	}
}

func TestFrontendComposesBorderlessPaneAndFocusedCursorWithoutEditingFrames(t *testing.T) {
	h := &host{cols: 90, rows: 16, surface: surfaceDocs, paneVisible: true, placement: placementLeft, focusPane: true, terminalFocused: true}
	layout := h.layout()
	h.frame = spikeFrame(layout.Shell.Dx(), layout.Shell.Dy())
	h.frame.Cells[0] = uv.Cell{Content: "S", Width: 1}
	p := &pane{kind: surfaceDocs, terminal: &terminal{}, frame: spikeFrame(layout.Body.Dx(), layout.Body.Dy())}
	h.pane = p
	p.frame.Cells[0] = uv.Cell{Content: "界", Width: 2}
	p.frame.Cells[1] = uv.Cell{}
	selection := uv.Style{Fg: color.RGBA{200, 210, 220, 255}, Bg: color.RGBA{70, 90, 120, 255}}
	p.frame.Cells[2] = uv.Cell{Content: "e\u0301", Width: 1, Style: selection}
	p.frame.Cells[3] = uv.Cell{Content: " ", Width: 1, Style: selection}
	p.frame.CursorX, p.frame.CursorY, p.frame.CursorStyle = 2, 1, 2
	originalShell, originalPane := append([]uv.Cell(nil), h.frame.Cells...), append([]uv.Cell(nil), p.frame.Cells...)
	view := h.View()
	if view.Cursor == nil || view.Cursor.X != layout.Body.Min.X+2 || view.Cursor.Y != layout.Body.Min.Y+1 || view.Cursor.Shape != tea.CursorUnderline {
		t.Fatalf("pane cursor offset/style = %+v", view.Cursor)
	}
	lines := strings.Split(ansi.Strip(view.Content), "\n")
	if !strings.HasPrefix(lines[0], "界e\u0301") || !strings.Contains(lines[h.rows-1], "docs | left | focus:pane | running") {
		t.Fatalf("child must start at the panel origin with status only in the footer: %q", lines)
	}
	if content := strings.Join(lines[:h.rows-1], "\n"); strings.ContainsAny(content, "┌┐└┘│─") || strings.Contains(content, "docs |") {
		t.Fatalf("host added a border/header/title strip: %q", content)
	}
	if !reflect.DeepEqual(h.frame.Cells, originalShell) || !reflect.DeepEqual(p.frame.Cells, originalPane) {
		t.Fatal("composition mutated a child's colors or snapshot")
	}
	h.focusPane = false
	if cursor := h.View().Cursor; cursor == nil || cursor.X != layout.Shell.Min.X || cursor.Y != layout.Shell.Min.Y {
		t.Fatalf("shell cursor offset = %+v", cursor)
	}
	h.placement = placementFloating
	layout = h.layout()
	h.frame = spikeFrame(layout.Shell.Dx(), layout.Shell.Dy())
	for y := layout.Body.Min.Y; y < layout.Body.Max.Y; y++ {
		for x := layout.Body.Min.X; x < layout.Body.Max.X; x++ {
			h.frame.Cells[y*h.frame.Cols+x] = uv.Cell{Content: "U", Width: 1, Style: uv.Style{Bg: color.RGBA{100, 30, 40, 255}}}
		}
	}
	originalShell = append([]uv.Cell(nil), h.frame.Cells...)
	h.frame.CursorX, h.frame.CursorY = layout.Body.Min.X, layout.Body.Min.Y
	view = h.View()
	if view.Cursor != nil || strings.Contains(ansi.Strip(view.Content), "U") {
		t.Fatal("floating pane failed to mask shell glyphs/cursor, including beyond its last frame's width")
	}
	outer := vt.NewEmulator(h.cols, h.rows)
	defer outer.Close()
	if _, err := outer.Write([]byte(strings.ReplaceAll(view.Content, "\n", "\r\n"))); err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]int{{layout.Body.Min.X, layout.Body.Min.Y}, {layout.Body.Min.X + 4, layout.Body.Min.Y}, {layout.Body.Max.X - 1, layout.Body.Max.Y - 1}} {
		if cell := outer.CellAt(point[0], point[1]); cell != nil && (cell.Style.Fg != nil || cell.Style.Bg != nil) {
			t.Fatalf("default-colored child cell at %v gained host foreground/background: %+v", point, cell)
		}
	}
	for _, x := range []int{2, 3} {
		cell := outer.CellAt(layout.Body.Min.X+x, layout.Body.Min.Y)
		if cell == nil || cell.Style.Fg == nil || cell.Style.Bg == nil ||
			color.RGBAModel.Convert(cell.Style.Fg) != color.RGBAModel.Convert(selection.Fg) ||
			color.RGBAModel.Convert(cell.Style.Bg) != color.RGBAModel.Convert(selection.Bg) {
			t.Fatalf("explicit child selection colors changed at column %d: %+v", x, cell)
		}
	}
	h.paneVisible = false
	if cursor := h.View().Cursor; cursor == nil || cursor.X != h.frame.CursorX || cursor.Y != h.frame.CursorY {
		t.Fatalf("hiding docs did not reveal the live shell cursor: %+v", cursor)
	}
	if !strings.Contains(ansi.Strip(h.View().Content), "U") || h.pane != p ||
		!reflect.DeepEqual(h.frame.Cells, originalShell) || !reflect.DeepEqual(p.frame.Cells, originalPane) {
		t.Fatal("hiding did not reveal the masked underlay or changed a child snapshot")
	}
}
