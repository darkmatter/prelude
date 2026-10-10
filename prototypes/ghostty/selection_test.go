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
	"golang.org/x/term"
)

// The ready file establishes that Bash's foreground child owns the real PTY,
// even while the test UI loop deliberately leaves Bash's C output unconsumed.
func TestHostSelectionPTYHelper(t *testing.T) {
	if os.Getenv("GHOSTTY_HOST_SELECTION_READER") != "1" {
		return
	}
	state, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("GHOSTTY_HOST_SELECTION_READY"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	fmt.Print("\x1b[?2004lRACE_FOREGROUND_READY\r\n")
	var input []byte
	for {
		var data [1]byte
		if _, err := os.Stdin.Read(data[:]); err != nil {
			t.Fatal(err)
		}
		if data[0] == '\r' || data[0] == '\n' {
			path := os.Getenv("GHOSTTY_HOST_SELECTION_INPUT")
			if err := os.WriteFile(path+".tmp", input, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(path+".tmp", path); err != nil {
				t.Fatal(err)
			}
			if err := term.Restore(int(os.Stdin.Fd()), state); err != nil {
				t.Fatal(err)
			}
			fmt.Print("RACE_FOREGROUND_FINISHED\r\n")
			os.Exit(0)
		}
		input = append(input, data[0])
	}
}

// Drive only the pane's actual process events, retaining any pending Bash
// output. This controls the event ordering without faking a successful exit.
func finishHostMenuWithoutShellOutput(t *testing.T, h *host) {
	t.Helper()
	p := h.pane
	if p == nil {
		t.Fatal("no menu pane to finish")
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for h.pane == p {
		if h.err != nil || h.paneError != nil {
			t.Fatalf("menu result: host=%v pane=%v", h.err, h.paneError)
		}
		select {
		case msg := <-p.process.output:
			h.Update(paneOutputMsg{pane: p, output: msg})
		case <-p.process.exited:
			if msg := p.nextOutput(); msg != nil {
				h.Update(msg)
			}
		case <-timer.C:
			t.Fatalf("waiting for real menu result; footer=%q", h.footer())
		}
	}
	if h.err != nil || h.pane != nil || h.paneError != nil {
		t.Fatalf("menu result did not close cleanly: host=%v pane=%v error=%v", h.err, h.pane, h.paneError)
	}
}

func TestHostMenuSelectionSubmissionRaces(t *testing.T) {
	for _, name := range []string{"AcceptLineBeforeCommandOutput", "PromptEndBeforeCommandStartInOneBatch"} {
		t.Run(name, func(t *testing.T) {
			frontendTools(t)
			dir := t.TempDir()
			ready, input, selected := filepath.Join(dir, "foreground.ready"), filepath.Join(dir, "foreground.input"), filepath.Join(dir, "selection.calls")
			t.Setenv("GHOSTTY_HOST_SELECTION_READY", ready)
			t.Setenv("GHOSTTY_HOST_SELECTION_INPUT", input)
			t.Setenv("GHOSTTY_FRONTEND_SELECTION", fmt.Sprintf("builtin printf 'selected\\n' >> %q; builtin printf 'RACE_SELECTED\\n'; false; builtin printf 'RACE_SELECTED_STATUS:%%s\\n' \"$?\"; false\n", selected))
			bash, err := exec.LookPath("bash")
			if err != nil {
				t.Fatal(err)
			}
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			wrapper := fmt.Sprintf("#!%s -p\nexport GHOSTTY_HOST_SELECTION_READER=1\nexec %q -test.run=^TestHostSelectionPTYHelper$\n", bash, binary)
			if err := os.WriteFile(filepath.Join(dir, "race-foreground"), []byte(wrapper), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			awaitFile := func(path string) []byte {
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

			h := newSpikeTestHost(t)
			h.Update(tea.PasteMsg{Content: "race-foreground"})
			awaitSpike(t, h, "foreground command in readline", func() bool { return spikeInput(h) == "race-foreground" })
			frontendChord(h, 'x')
			awaitFrontendReady(t, h, "MENU")
			frontendChord(h, tea.KeyTab)
			spikeKey(h, tea.KeyEnter, 0)
			awaitFile(ready)

			frontendChord(h, tea.KeyTab)
			spikeKey(h, tea.KeyEnter, 0)
			finishHostMenuWithoutShellOutput(t, h)

			if name == "PromptEndBeforeCommandStartInOneBatch" {
				// Replay a coalesced PTY batch: D permits a new command, but the
				// later C invalidates B. No handoff may run between its markers.
				h.Update(outputMsg{data: []byte("\x1b]133;D;0\a\x1b]133;A\aspike $ \x1b]133;B\a\x1b]133;C\a")})
			}
			const userInput = "only-user-input"
			for _, key := range userInput {
				spikeKey(h, key, 0)
			}
			spikeKey(h, tea.KeyEnter, 0)
			if got := string(awaitFile(input)); got != userInput {
				t.Fatalf("menu handoff injected command bytes into the foreground PTY: got %q, want %q", got, userInput)
			}
			awaitSpike(t, h, "queued selection completes after the real primary prompt", func() bool {
				return h.prompt.ready && spikeInput(h) == "" &&
					strings.Contains(spikeFrameText(h.frame), "RACE_SELECTED_STATUS:1")
			})
			if got := string(awaitFile(selected)); got != "selected\n" {
				t.Fatalf("queued selection ran more than once: %q", got)
			}
		})
	}

	t.Run("QuotedEnterKeepsParkedInput", func(t *testing.T) {
		frontendTools(t)
		dir := t.TempDir()
		selected, parkedOutput := filepath.Join(dir, "selection.calls"), filepath.Join(dir, "parked.input")
		t.Setenv("GHOSTTY_PARKED", parkedOutput)
		t.Setenv("GHOSTTY_FRONTEND_SELECTION", fmt.Sprintf("builtin printf 'selected\\n' >> %q; builtin printf 'RACE_SELECTED\\n'; false; builtin printf 'RACE_SELECTED_STATUS:%%s\\n' \"$?\"; false\n", selected))
		h := newSpikeTestHost(t)
		const prefix = "printf '%s' 'quoted"
		const suffix = "enter' > \"$GHOSTTY_PARKED\""
		const parked = prefix + "^M" + suffix
		h.Update(tea.PasteMsg{Content: prefix})
		awaitSpike(t, h, "input before quoted Enter", func() bool { return spikeInput(h) == prefix })
		spikeKey(h, 'v', tea.ModCtrl)
		spikeKey(h, tea.KeyEnter, 0)
		h.Update(tea.PasteMsg{Content: suffix})
		awaitSpike(t, h, "literal Enter remains editable in readline", func() bool { return spikeInput(h) == parked })
		frontendChord(h, 'x')
		awaitFrontendReady(t, h, "MENU")
		spikeKey(h, tea.KeyEnter, 0)
		awaitFrontend(t, h, "selection runs without cancelling the quoted parked line", func() bool {
			return h.pane == nil && h.prompt.ready && spikeInput(h) == parked &&
				strings.Contains(spikeFrameText(h.frame), "RACE_SELECTED_STATUS:1")
		})
		if data, err := os.ReadFile(selected); err != nil || string(data) != "selected\n" {
			t.Fatalf("selection did not run exactly once with quoted Enter parked: %q, %v", data, err)
		}
		spikeKey(h, tea.KeyEnter, 0)
		awaitSpike(t, h, "restored literal input executes normally", func() bool {
			return h.prompt.ready && spikeInput(h) == ""
		})
		if data, err := os.ReadFile(parkedOutput); err != nil || string(data) != "quoted\renter" {
			t.Fatalf("parked literal Enter changed across handoff: %q, %v", data, err)
		}
		spikeShellStatus(t, h, 0)
	})
}
