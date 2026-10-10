package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func newMotdHandoffHost(t *testing.T, starship bool, calls string) *host {
	t.Helper()
	dir := frontendTools(t)
	writeMotdFixture(t, dir, fmt.Sprintf("builtin printf 'args:%%s\\n' \"$*\" >> %q; %s", calls, fixtureMotd))
	home := t.TempDir()
	env := smokeEnvironment(home)
	if starship {
		config := filepath.Join(home, "starship.toml")
		if err := os.WriteFile(config, []byte(smokeStarshipConfig), 0o600); err != nil {
			t.Fatal(err)
		}
		env = append(env, "STARSHIP_CONFIG="+config)
	}
	// Both startup and the shortcut must use the public PATH command, not an
	// inherited shell function with the same name.
	env = append(env, `BASH_FUNC_motd%%=() { builtin printf 'SHADOW_MOTD\n'; }`)
	h, err := newHost(160, 18, env, starship)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := h.close(); err != nil {
			t.Error(err)
		}
	})
	awaitSpike(t, h, "initial prompt with recorded MOTD", func() bool { return h.prompt.ready && spikeInput(h) == "" })
	assertMotdCalls(t, calls, 1)
	return h
}

func awaitMotdHandoffFile(t *testing.T, path string) []byte {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			return data
		}
		if !os.IsNotExist(err) || time.Now().After(deadline) {
			t.Fatalf("waiting for %s: %v", filepath.Base(path), err)
		}
		time.Sleep(outputPace)
	}
}

func motdHandoffOutput(frame terminalFrame) string {
	rows := strings.Split(spikeFrameText(frame), "\n")
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	return strings.Join(rows, "\n")
}

func TestHostMotdPreservesParkedLinePointAndMark(t *testing.T) {
	for _, starship := range []bool{false, true} {
		name := "Bash"
		if starship {
			name = "Starship"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			calls, result := filepath.Join(dir, "motd.calls"), filepath.Join(dir, "parked.result")
			h := newMotdHandoffHost(t, starship, calls)
			spikeKey(h, 'd', tea.ModAlt)
			awaitFrontendReady(t, h, "DOCS")
			docs, process, engine := h.pane, h.pane.process, h.pane.terminal
			spikeKey(h, ']', tea.ModCtrl)
			frontendChord(h, tea.KeyTab)
			paneSize := frontendPTYSize(t, process)
			parked := "printf '%s' 'alpha-beta-界🙂' > " + shellQuote(result)
			h.Update(tea.PasteMsg{Content: parked})
			awaitSpike(t, h, "wide parked input before MOTD", func() bool { return spikeInput(h) == parked })
			point := func() int {
				if h.prompt.anchor < 0 {
					return -1
				}
				return h.frame.CursorY*h.frame.Cols + h.frame.CursorX - h.prompt.anchor
			}
			const markPrefix = "printf '%s' 'alpha"
			spikeKey(h, 'a', tea.ModCtrl)
			for range len(markPrefix) {
				spikeKey(h, tea.KeyRight, 0)
			}
			awaitSpike(t, h, "Readline point before setting mark", func() bool { return spikeInput(h) == parked && point() == len(markPrefix) })
			spikeKey(h, ' ', tea.ModCtrl)
			for range len("-beta") {
				spikeKey(h, tea.KeyRight, 0)
			}
			awaitSpike(t, h, "mid-line point distinct from mark", func() bool { return point() == len(markPrefix+"-beta") })
			if starship {
				frontendChord(h, 'm')
			} else {
				spikeKey(h, 'm', tea.ModAlt)
			}
			awaitFrontend(t, h, "MOTD handoff restores exact parked line and point", func() bool {
				data, err := os.ReadFile(calls)
				return err == nil && string(data) == "args:\nargs:\n" && h.prompt.ready && !h.restoreInput && !h.shellAcceptPending &&
					spikeInput(h) == parked && point() == len(markPrefix+"-beta")
			})
			assertMotdCalls(t, calls, 2)
			view := h.View()
			if h.leader || h.pane != docs || h.paneVisible || h.focusPane || docs.closed || docs.process != process || docs.terminal != engine ||
				frontendPTYSize(t, process) != paneSize || frontendPTYSize(t, h.child) != [2]int{160, 17} || h.layout().FooterY != 17 ||
				view.Cursor == nil || view.Cursor.X != h.frame.CursorX || view.Cursor.Y != h.frame.CursorY ||
				strings.Contains(frontendCompactText(docs.frame), fmt.Sprintf("%x", "command motd")) ||
				strings.Contains(spikeFrameText(h.frame), "SHADOW_MOTD") {
				t.Fatalf("MOTD did not stay in the full main shell with the original pane retained: cursor=%+v\n%s", view.Cursor, ansi.Strip(view.Content))
			}
			// Exchange point and mark in real Readline. Insertion at the old mark
			// proves that the handoff restored more than just the visible caret.
			spikeKey(h, 'x', tea.ModCtrl)
			spikeKey(h, 'x', tea.ModCtrl)
			awaitSpike(t, h, "parked mark survives MOTD", func() bool { return spikeInput(h) == parked && point() == len(markPrefix) })
			spikeKey(h, 'Z', 0)
			wantLine := strings.Replace(parked, "alpha-beta", "alphaZ-beta", 1)
			awaitSpike(t, h, "editing at restored mark", func() bool { return spikeInput(h) == wantLine })
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "restored wide input executes normally", func() bool {
				data, err := os.ReadFile(result)
				return err == nil && string(data) == "alphaZ-beta-界🙂" && h.prompt.ready && spikeInput(h) == ""
			})
			spikeShellStatus(t, h, 0)
		})
	}
}

func TestHostMotdForegroundSubmissionRaces(t *testing.T) {
	for _, name := range []string{"AcceptLineBeforeCommandOutput", "PromptEndBeforeCommandStartInOneBatch"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			ready, input, calls := filepath.Join(dir, "foreground.ready"), filepath.Join(dir, "foreground.input"), filepath.Join(dir, "motd.calls")
			t.Setenv("GHOSTTY_HOST_SELECTION_READY", ready)
			t.Setenv("GHOSTTY_HOST_SELECTION_INPUT", input)
			bash, err := exec.LookPath("bash")
			if err != nil {
				t.Fatal(err)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			wrapper := fmt.Sprintf("#!%s -p\nexport GHOSTTY_HOST_SELECTION_READER=1\nexec %q -test.run=^TestHostSelectionPTYHelper$\n", bash, binary)
			if err := os.WriteFile(filepath.Join(dir, "motd-foreground"), []byte(wrapper), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			h := newMotdHandoffHost(t, false, calls)
			h.Update(tea.PasteMsg{Content: "motd-foreground"})
			awaitSpike(t, h, "foreground command before acceptance", func() bool { return spikeInput(h) == "motd-foreground" })
			spikeKey(h, tea.KeyEnter, 0)
			awaitMotdHandoffFile(t, ready)
			spikeKey(h, 'm', tea.ModAlt)
			if name == "PromptEndBeforeCommandStartInOneBatch" {
				h.Update(outputMsg{data: []byte("\x1b]133;D;0\a\x1b]133;A\aprelude $ \x1b]133;B\a\x1b]133;C\a")})
			}
			if h.pendingCommand != "command motd\n" {
				t.Fatal("foreground MOTD request was dispatched instead of queued")
			}
			assertMotdCalls(t, calls, 1)
			const userInput = "only-user-input"
			for _, code := range userInput {
				spikeKey(h, code, 0)
			}
			spikeKey(h, tea.KeyEnter, 0)
			if got := string(awaitMotdHandoffFile(t, input)); got != userInput {
				t.Fatalf("MOTD injected private command bytes into the foreground PTY: %q", got)
			}
			awaitSpike(t, h, "queued MOTD runs after the real primary prompt", func() bool {
				data, err := os.ReadFile(calls)
				return err == nil && string(data) == "args:\nargs:\n" && h.prompt.ready && !h.restoreInput && !h.shellAcceptPending && spikeInput(h) == ""
			})
			assertMotdCalls(t, calls, 2)
			if !strings.Contains(spikeFrameText(h.frame), "RACE_FOREGROUND_FINISHED") || !strings.Contains(spikeFrameText(h.frame), "MOTD_SHELL:") {
				t.Fatal("foreground completion or MOTD output disappeared from the native shell")
			}
		})
	}
}

func TestHostMotdPS2SafetyAndPendingSlot(t *testing.T) {
	for _, starship := range []bool{false, true} {
		name := "Bash"
		if starship {
			name = "Starship"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			calls, selected := filepath.Join(dir, "motd.calls"), filepath.Join(dir, "selection.calls")
			t.Setenv("GHOSTTY_FRONTEND_SELECTION", fmt.Sprintf("builtin printf 'selected\\n' >> %q; builtin printf 'PS2_SELECTION_RAN\\n'\n", selected))
			h := newMotdHandoffHost(t, starship, calls)
			h.Update(tea.PasteMsg{Content: "printf '%s\\n' 'PS2_LEFT"})
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "real secondary prompt owns input", func() bool {
				return !h.prompt.ready && strings.Contains(spikeFrameText(h.frame), "PS2_LEFT")
			})
			spikeKey(h, 'm', tea.ModAlt)
			if h.pendingCommand != "command motd\n" {
				t.Fatal("MOTD was not safely queued at PS2")
			}
			assertMotdCalls(t, calls, 1)
			h.Update(tea.PasteMsg{Content: "_RIGHT'"})
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "PS2 command finishes before queued MOTD", func() bool {
				data, err := os.ReadFile(calls)
				return err == nil && string(data) == "args:\nargs:\n" && h.prompt.ready && !h.restoreInput && !h.shellAcceptPending && spikeInput(h) == ""
			})
			if !strings.Contains(motdHandoffOutput(h.frame), "PS2_LEFT\n_RIGHT") {
				t.Fatalf("MOTD bytes altered the real quoted continuation output:\n%s", spikeFrameText(h.frame))
			}
			// Queue a real picker result at the next PS2. A later MOTD shortcut
			// must reject the occupied slot, not replace or run ahead of it.
			h.Update(tea.PasteMsg{Content: "printf '%s\\n' 'PS2_SECOND"})
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "second secondary prompt", func() bool { return !h.prompt.ready && strings.Contains(spikeFrameText(h.frame), "PS2_SECOND") })
			spikeKey(h, 'x', tea.ModAlt)
			awaitFrontendReady(t, h, "MENU")
			spikeKey(h, tea.KeyEnter, 0)
			awaitFrontend(t, h, "picker result queues without entering PS2", func() bool { return h.pane == nil && h.pendingCommand != "" })
			spikeKey(h, 'm', tea.ModAlt)
			if !strings.Contains(ansi.Strip(h.View().Content), "MOTD not queued") {
				t.Fatal("occupied-slot MOTD rejection was not observable")
			}
			if _, err := os.Stat(selected); !os.IsNotExist(err) {
				t.Fatalf("queued selection leaked into PS2: %v", err)
			}
			h.Update(tea.PasteMsg{Content: "_DONE'"})
			spikeKey(h, tea.KeyEnter, 0)
			awaitSpike(t, h, "original pending selection runs at the primary prompt", func() bool {
				data, err := os.ReadFile(selected)
				return err == nil && string(data) == "selected\n" && h.prompt.ready && !h.restoreInput && !h.shellAcceptPending && spikeInput(h) == ""
			})
			assertMotdCalls(t, calls, 2)
			if !strings.Contains(motdHandoffOutput(h.frame), "PS2_SECOND\n_DONE") || !strings.Contains(spikeFrameText(h.frame), "PS2_SELECTION_RAN") {
				t.Fatal("pending-slot rejection changed continuation output or discarded the picker result")
			}
		})
	}
}
