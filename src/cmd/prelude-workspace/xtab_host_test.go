package main

import (
	"flag"
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

const xtabOptIn = xtabProtoEnv + "=1"

// xtabTools replaces the routing fixture's x with a stand-in for the real
// picker that speaks the prototype's menu half: it acks readiness, shows each
// seed it is triggered to read, and publishes one filter edit on "p". Run
// from Bash (no --embedded) it only reports its arguments.
func xtabTools(t *testing.T) {
	t.Helper()
	dir := frontendTools(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("#!%s\nexport XTAB_MENU_HELPER=1\nexec %q -test.run=^TestXtabMenuHelper$ -- \"$@\"\n", bash, binary)
	if err := os.WriteFile(filepath.Join(dir, "x"), []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestXtabMenuHelper(t *testing.T) {
	if os.Getenv("XTAB_MENU_HELPER") == "" {
		return
	}
	args := flag.Args()
	if len(args) == 0 || args[0] != "--embedded" {
		fmt.Printf("XRAN:%s\n", strings.Join(args, ","))
		os.Exit(0)
	}
	if _, err := term.MakeRaw(int(os.Stdin.Fd())); err != nil {
		t.Fatal(err)
	}
	fmt.Print(xtabProtoReadyOSC + "XMENU_READY\r\n")
	for {
		var data [1]byte
		if _, err := os.Stdin.Read(data[:]); err != nil {
			os.Exit(0)
		}
		switch data[0] {
		case xtabProtoTrigger:
			seed, err := os.ReadFile(os.Getenv(xtabProtoEnvIn))
			if err != nil {
				t.Fatal(err)
			}
			fmt.Printf("SEED<%s>", strings.ReplaceAll(string(seed), "\n", "|"))
		case 'p':
			if err := os.WriteFile(os.Getenv(xtabProtoEnvOut), []byte("3\nbux"), 0o600); err != nil {
				t.Fatal(err)
			}
			fmt.Print(xtabProtoOutOSC + "PUBLISHED")
		default:
			fmt.Printf("KEY<%02x>", data[0])
		}
	}
}

func xtabType(h *host, text string) {
	for _, r := range text {
		h.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// xtabSetups numbers setup commands so each one's completion marker is new
// on screen; the typed command never contains the marker it prints.
var xtabSetups int

// xtabRun submits one setup command and waits for the next empty prompt.
func xtabRun(t *testing.T, h *host, command string) {
	t.Helper()
	xtabSetups++
	h.Update(tea.PasteMsg{Content: fmt.Sprintf("%s; printf 'SETUP%%sDONE\\n' %d", command, xtabSetups)})
	spikeKey(h, tea.KeyEnter, 0)
	marker := fmt.Sprintf("SETUP%dDONE", xtabSetups)
	awaitSpike(t, h, "setup: "+command, func() bool {
		return h.prompt.ready && spikeInput(h) == "" && strings.Contains(spikeFrameText(h.frame), marker)
	})
}

// xtabFocus moves input to the picker or back to Bash with Alt+Tab.
func xtabFocus(t *testing.T, h *host, picker bool) {
	t.Helper()
	if h.paneFocused() != picker {
		spikeKey(h, tea.KeyTab, tea.ModAlt)
	}
	if h.paneFocused() != picker {
		t.Fatalf("focus on picker = %t, want %t", h.paneFocused(), picker)
	}
}

func xtabDraft(t *testing.T, h *host, line string) {
	t.Helper()
	h.Update(tea.PasteMsg{Content: line})
	awaitSpike(t, h, "draft "+line, func() bool { return spikeInput(h) == line })
}

func xtabPaneShows(h *host, text string) bool {
	return h.pane != nil && strings.Contains(frontendCompactText(h.pane.frame), text)
}

// xtabOpenSeeded types `x bu`, presses Tab, and waits for the picker to show
// the seed it read after its trigger byte.
func xtabOpenSeeded(t *testing.T, h *host) {
	t.Helper()
	xtabDraft(t, h, "x bu")
	spikeKey(h, tea.KeyTab, 0)
	awaitFrontend(t, h, "picker seeded from the Readline line", func() bool {
		return xtabPaneShows(h, "SEED<2|bu>")
	})
}

func TestXtabTabWithoutOptInIsBashCompletion(t *testing.T) {
	frontendTools(t)
	t.Setenv(xtabProtoEnv, "") // registers the restore; the test needs it unset
	if err := os.Unsetenv(xtabProtoEnv); err != nil {
		t.Fatal(err)
	}
	h := newSpikeTestHost(t)
	xtabRun(t, h, "complete -W bundle x")
	xtabDraft(t, h, "x bu")
	spikeKey(h, tea.KeyTab, 0)
	awaitSpike(t, h, "Bash's own completion of the x argument", func() bool {
		return strings.TrimRight(spikeInput(h), " ") == "x bundle"
	})
	if h.pane != nil || h.surface != surfaceNone {
		t.Fatalf("Tab opened a %s pane without the opt-in", h.surface)
	}
}

// Finding: under `set -C` a plain > refused to rewrite the snapshot, the ack
// was sent anyway, and the Tab vanished. The same session then proves the
// return path: a picker edit lands in Readline.
func TestXtabSyncsUnderNoclobber(t *testing.T) {
	xtabTools(t)
	h := newSpikeTestHost(t, xtabOptIn)
	xtabRun(t, h, "set -C")
	xtabOpenSeeded(t, h)
	xtabFocus(t, h, true)
	xtabType(h, "p")
	awaitFrontend(t, h, "picker edit applied to the Readline line", func() bool {
		return spikeInput(h) == "x bux"
	})
}

func TestXtabUnreadableSnapshotReplaysNativeTab(t *testing.T) {
	xtabTools(t)
	h := newSpikeTestHost(t, xtabOptIn)
	xtabRun(t, h, "complete -W bundle x")
	xtabRun(t, h, `__prelude_xtab_proto_snapshot() { builtin printf junk >| "$__prelude_xtab_proto_dir/snapshot"; builtin printf '\e]133;X\a'; }`)
	xtabDraft(t, h, "x bu")
	spikeKey(h, tea.KeyTab, 0)
	awaitSpike(t, h, "native completion after the failed snapshot read", func() bool {
		return strings.TrimRight(spikeInput(h), " ") == "x bundle"
	})
	if h.pane != nil || !strings.HasPrefix(h.notice, "x-tab prototype: ") {
		t.Fatalf("failed read must report and open nothing: pane=%v notice=%q", h.pane, h.notice)
	}
}

// A private key sent while Readline waits for the rest of a key sequence
// would complete that sequence and insert "[994~" as text.
func TestXtabTabAfterSequencePrefixStaysNative(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"esc", tea.KeyPressMsg{Code: tea.KeyEscape}},
		{"ctrl+x", tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xtabTools(t)
			h := newSpikeTestHost(t, xtabOptIn)
			xtabDraft(t, h, "x bu")
			h.Update(tc.key)
			spikeKey(h, tea.KeyTab, 0)
			xtabType(h, "z")
			awaitSpike(t, h, "line intact and later input not held", func() bool { return spikeInput(h) == "x buz" })
			if h.pane != nil {
				t.Fatal("Tab inside a key sequence opened the picker")
			}
		})
	}
}

// Between a submit key and its OSC 133;C the prompt still looks ready. A
// picker edit arriving in that window must not be applied: its private key
// would queue behind the submit key and rewrite the next prompt's line.
func TestXtabSubmitKeysDetachBeforeCommandStart(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  tea.KeyPressMsg
	}{
		{"enter", tea.KeyPressMsg{Code: tea.KeyEnter}},
		{"ctrl+j", tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}},
		// A distinct Ctrl+M is absent: the engine encodes it as CSI u, which
		// Bash does not accept as a line. Legacy terminals report it as Enter.
		{"ctrl+o", tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xtabTools(t)
			h := newSpikeTestHost(t, xtabOptIn)
			xtabOpenSeeded(t, h)
			xtabFocus(t, h, false)
			h.Update(tc.key)
			xtabFocus(t, h, true)
			xtabType(h, "p")
			// Only the picker is pumped: Bash's OSC 133;C stays unread.
			p, deadline := h.pane, time.After(5*time.Second)
			for !xtabPaneShows(h, "PUBLISHED") {
				select {
				case msg := <-p.process.output:
					h.Update(paneOutputMsg{pane: p, output: msg})
				case <-deadline:
					t.Fatalf("picker never published:\n%s", spikeFrameText(p.frame))
				}
			}
			xtabFocus(t, h, false)
			xtabType(h, "z")
			// Ctrl+O may recall the submitted line; no key may apply "bux".
			awaitFrontend(t, h, "submitted line ran and the picker edit was dropped", func() bool {
				input := spikeInput(h)
				return strings.Contains(spikeFrameText(h.frame), "XRAN:bu") &&
					strings.HasSuffix(input, "z") && !strings.Contains(input, "bux")
			})
		})
	}
}

// A key the host cannot know submits the line (here a user binding) lets a
// Tab be swallowed after the command already started. Input typed behind
// that Tab is the user's type-ahead and must survive the command start.
func TestXtabCommandStartReleasesInputHeldBehindTab(t *testing.T) {
	xtabTools(t)
	h := newSpikeTestHost(t, xtabOptIn)
	xtabRun(t, h, `bind '"\C-t": accept-line'`)
	xtabDraft(t, h, `printf 'RAN%s\n' OK`)
	spikeKey(h, 't', tea.ModCtrl)
	spikeKey(h, tea.KeyTab, 0)
	xtabType(h, "q")
	awaitSpike(t, h, "held input released when the command started", func() bool {
		return strings.Contains(spikeFrameText(h.frame), "RANOK") && spikeInput(h) == "q"
	})
}

// The routing fixture never acks readiness, like a picker that failed to
// start: input typed into it must not stay queued behind the seed forever.
func TestXtabUnreadyMenuReleasesHeldInputOnTimeout(t *testing.T) {
	frontendTools(t)
	h := newSpikeTestHost(t, xtabOptIn)
	xtabDraft(t, h, "x bu")
	spikeKey(h, tea.KeyTab, 0)
	awaitFrontendReady(t, h, "MENU")
	xtabFocus(t, h, true)
	xtabType(h, "ab")
	h.Update(xtabProtoTimeoutMsg{gen: h.proto.gen})
	awaitFrontend(t, h, "held keys reach the unseeded picker", func() bool {
		return xtabPaneShows(h, "INPUT:6162")
	})
	if !strings.HasPrefix(h.notice, "x-tab prototype: ") {
		t.Fatalf("timeout must be reported, notice=%q", h.notice)
	}
}
