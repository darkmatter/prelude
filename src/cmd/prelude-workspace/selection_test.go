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

func TestHostMenuSelectionDroppedWhileForegroundRuns(t *testing.T) {
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
	spikeKey(h, tea.KeyEnter, 0)
	awaitSpike(t, h, "foreground job owns the PTY and the host absorbed OSC133 C", func() bool {
		_, err := os.Stat(ready)
		return err == nil && h.prompt.phase == "running" && !h.prompt.ready
	})
	frontendAlt(h, 'x')
	awaitFrontendReady(t, h, "MENU")
	spikeKey(h, tea.KeyEnter, 0)
	awaitFrontend(t, h, "picker result closes the menu while Bash is busy", func() bool { return h.pane == nil })
	if h.notice != "Shell busy; command not run" {
		t.Fatalf("busy selection was not dropped with a notice: notice=%q", h.notice)
	}
	if _, err := os.Stat(selected); !os.IsNotExist(err) {
		t.Fatalf("busy selection ran instead of being dropped: %v", err)
	}
	const userInput = "only-user-input"
	for _, key := range userInput {
		spikeKey(h, key, 0)
	}
	spikeKey(h, tea.KeyEnter, 0)
	if got := string(awaitFile(input)); got != userInput {
		t.Fatalf("menu handoff injected command bytes into the foreground PTY: got %q, want %q", got, userInput)
	}
	awaitSpike(t, h, "real primary prompt after the foreground job", func() bool {
		return h.prompt.ready && spikeInput(h) == ""
	})
	if _, err := os.Stat(selected); !os.IsNotExist(err) {
		t.Fatalf("dropped selection ran at the primary prompt: %v", err)
	}
	if strings.Contains(spikeFrameText(h.frame), "RACE_SELECTED") {
		t.Fatal("dropped selection produced output at the primary prompt")
	}
}

func TestHostMenuSelectionAtPrompt(t *testing.T) {

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
		frontendAlt(h, 'x')
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
