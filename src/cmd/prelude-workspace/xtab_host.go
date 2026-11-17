package main

// x-tab prototype, host half: the Tab decision at Bash's prompt, the queue
// that holds input behind an undecided Tab, and the snapshot follow-ups of an
// attached session. xtab_protocol.go documents the protocol; xtab_pane.go
// owns the menu pane side.

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

type xtabPrototype struct {
	enabled       bool
	active        bool
	pane          *pane // attached menu pane; any other pane never syncs
	prefix        string
	tabPending    bool
	snapshotReady bool
	pendingCmd    tea.Cmd
	pendingSeed   *xtabProtoSeed
	outScan       xtabProtoNeedle
	readyScan     xtabProtoNeedle
	// nativeTabRun: the last Tab fell back to native completion, so the next
	// Tab must reach Bash with no bind-x in between. Any other Bash-bound
	// key or paste clears it.
	nativeTabRun bool
	// tabUnsafe: the previous Bash-bound key makes a private key unsafe, so
	// the next Tab stays native. A lone Esc or Ctrl+X may have opened a
	// Readline key sequence that ESC[994~ would extend (inserting "[994~" as
	// text), and a submit key may already have handed the terminal to a
	// command. Any other Bash-bound key or paste clears it.
	tabUnsafe bool
	// held queues Bash-/menu-bound input in arrival order while a Tab
	// snapshot (or a fresh pane's ready ack) is outstanding. The two waits
	// never overlap, so the queue has one destination at a time.
	held []xtabProtoHeld
	// gen identifies the most recently armed one-shot timeout (Tab snapshot
	// or fresh-pane ready wait); an older timer firing later is ignored.
	gen int
}

type xtabProtoHeld struct {
	key     tea.KeyPressMsg
	paste   string
	isPaste bool
}

type xtabProtoTimeoutMsg struct{ gen int }

func newXtabPrototype(env []string) xtabPrototype {
	return xtabPrototype{
		enabled:   xtabProtoOptIn(env),
		outScan:   xtabProtoNeedle{seq: xtabProtoOutOSC},
		readyScan: xtabProtoNeedle{seq: xtabProtoReadyOSC},
	}
}

func (h *host) protoReady() bool {
	return h.proto.enabled && h.child != nil && h.child.protoDir != ""
}

func (h *host) protoAttached() bool {
	return h.proto.active && h.proto.pane != nil && h.proto.pane == h.pane &&
		h.currentPane(h.pane) && h.pane.protoInFile != ""
}

func (h *host) protoDetach() {
	if h.proto.pendingSeed != nil {
		// Input held for a fresh pane has no destination once its seed is
		// abandoned; callers that can still deliver it flush first.
		h.proto.held = nil
	}
	h.proto.active, h.proto.pane, h.proto.pendingSeed = false, nil, nil
}

// protoArmTimeout starts a one-shot lost-ack escape (not polling) for the
// wait the caller just entered.
func (h *host) protoArmTimeout(wait time.Duration) tea.Cmd {
	h.proto.gen++
	gen := h.proto.gen
	return tea.Tick(wait, func(time.Time) tea.Msg {
		return xtabProtoTimeoutMsg{gen: gen}
	})
}

// protoTimeout resolves whichever wait the newest timer was armed for; a
// wait that already ended leaves nothing pending and the timer is a no-op.
func (h *host) protoTimeout(gen int) tea.Cmd {
	if !h.proto.enabled || gen != h.proto.gen {
		return nil
	}
	switch {
	case h.proto.tabPending:
		// The snapshot's bind-x never ran or could not write, so no ack is
		// coming. Resolve like a native decision and detach, since the
		// snapshot channel just proved unreliable. A late ack is then
		// harmless: with no Tab pending and no session it decides nothing.
		h.proto.tabPending = false
		h.protoDetach()
		if err := h.protoNativeTab(); err != nil {
			return h.fail(err)
		}
	case h.proto.pendingSeed != nil:
		h.notice = "x-tab prototype: menu did not become ready"
		h.protoAbandonSeed()
	}
	return nil
}

// protoTab intercepts one plain Tab destined for Bash. It reports handled
// only when the snapshot request was sent; anything else keeps native Tab.
func (h *host) protoTab() (tea.Cmd, bool) {
	if !h.protoReady() || h.paneFocused() || !h.prompt.ready || h.restoreInput ||
		h.proto.nativeTabRun || h.proto.tabUnsafe || h.proto.pendingSeed != nil {
		return nil, false
	}
	if err := h.child.send([]byte(xtabProtoSnapshot)); err != nil {
		return h.fail(err), true
	}
	h.proto.tabPending, h.proto.held = true, nil
	return h.protoArmTimeout(xtabProtoTabTimeout), true
}

// protoNativeTab resolves an undecided Tab natively: replay the user's Tab
// (never into a program that started, or a menu handoff that began, while
// the snapshot was in flight), then release everything held behind it.
func (h *host) protoNativeTab() error {
	h.proto.nativeTabRun = false
	if h.child != nil && h.prompt.ready && !h.restoreInput {
		if err := h.child.send([]byte("\t")); err != nil {
			return err
		}
		h.proto.nativeTabRun = true
	}
	return h.protoFlushHeldToBash()
}

// protoHoldShellKey queues a Bash-bound key while a Tab decision is in
// flight; nothing may overtake the replayed Tab or the menu seed.
func (h *host) protoHoldShellKey(message tea.KeyPressMsg) bool {
	if !h.protoReady() || !h.proto.tabPending {
		return false
	}
	h.proto.held = append(h.proto.held, xtabProtoHeld{key: message})
	return true
}

func (h *host) protoHoldShellPaste(content string) bool {
	if !h.protoReady() || !h.proto.tabPending {
		return false
	}
	h.proto.held = append(h.proto.held, xtabProtoHeld{paste: content, isPaste: true})
	return true
}

// protoCtrlCEscape gives Ctrl+C an immediate way out of a Tab decision whose
// ack may never come: the pending Tab is abandoned (not replayed), held
// input is released to Bash in order, and the caller then forwards the
// Ctrl+C itself through the normal path. Returns a failure command only if
// releasing held input fails.
func (h *host) protoCtrlCEscape(message tea.KeyPressMsg) tea.Cmd {
	if !h.protoReady() || !h.proto.tabPending || message.String() != "ctrl+c" {
		return nil
	}
	h.proto.tabPending, h.proto.nativeTabRun = false, false
	h.protoDetach()
	if err := h.protoFlushHeldToBash(); err != nil {
		return h.fail(err)
	}
	return nil
}

// protoNoteShellKey records what a key just sent to Bash means for the next
// Tab: only another Tab continues a native completion run, and a key that
// may leave Readline mid-sequence or submit the line keeps that Tab native.
func (h *host) protoNoteShellKey(message tea.KeyPressMsg) {
	if message.String() != "tab" {
		h.proto.nativeTabRun = false
	}
	h.proto.tabUnsafe = xtabProtoPrefixKey(message) || xtabProtoSubmitKey(message)
}

// protoFlushHeldToBash releases held user input to Bash in arrival order,
// encoded by the shell terminal, with no private key interleaved. It keeps
// the next-Tab bookkeeping but deliberately performs no attached-session
// snapshots.
func (h *host) protoFlushHeldToBash() error {
	held := h.proto.held
	h.proto.held = nil
	for _, item := range held {
		var data []byte
		var err error
		if item.isPaste {
			h.proto.nativeTabRun, h.proto.tabUnsafe = false, false
			data, err = h.terminal.Paste(item.paste)
		} else {
			h.protoNoteShellKey(item.key)
			data, err = h.terminal.Key(item.key.Key())
		}
		if err != nil {
			return fmt.Errorf("libghostty-vt input: %w", err)
		}
		if err := h.child.send(data); err != nil {
			return err
		}
	}
	return nil
}

// protoAfterShellKey follows simple forwarded editing keys with a snapshot
// request while attached. A submit key detaches at once rather than at its
// OSC 133;C: until that marker arrives the prompt still looks ready, and a
// menu edit applied in that window would reach the command instead of
// Readline. No private key may ever reach a running program.
func (h *host) protoAfterShellKey(message tea.KeyPressMsg, cmd tea.Cmd) tea.Cmd {
	if !h.protoReady() {
		return cmd
	}
	h.protoNoteShellKey(message)
	if !h.protoAttached() || h.err != nil {
		return cmd
	}
	// Incremental history search is modal Readline state; detaching keeps a
	// snapshot key out of it entirely.
	if s := message.String(); xtabProtoSubmitKey(message) || s == "ctrl+r" || s == "ctrl+s" {
		h.protoDetach()
		return cmd
	}
	if !xtabProtoEditKey(message) {
		return cmd
	}
	if err := h.child.send([]byte(xtabProtoSnapshot)); err != nil {
		return h.fail(err)
	}
	return cmd
}

func (h *host) protoAfterShellPaste(cmd tea.Cmd) tea.Cmd {
	if !h.protoReady() {
		return cmd
	}
	h.proto.nativeTabRun, h.proto.tabUnsafe = false, false
	if !h.protoAttached() || h.err != nil {
		return cmd
	}
	if err := h.child.send([]byte(xtabProtoSnapshot)); err != nil {
		return h.fail(err)
	}
	return cmd
}

// xtabProtoSubmitKey reports the keys Readline's default keymap binds to
// accepting the line (accept-line and operate-and-get-next).
func xtabProtoSubmitKey(message tea.KeyPressMsg) bool {
	switch message.String() {
	case "enter", "ctrl+j", "ctrl+m", "ctrl+o":
		return true
	}
	return false
}

// xtabProtoPrefixKey reports keys that start a multi-key Readline sequence
// in the default keymap: Esc (and Ctrl+[, which Bash receives as ESC) is the
// Meta prefix, Ctrl+X a prefix map.
func xtabProtoPrefixKey(message tea.KeyPressMsg) bool {
	switch message.String() {
	case "esc", "ctrl+[", "ctrl+x":
		return true
	}
	return false
}

func xtabProtoEditKey(message tea.KeyPressMsg) bool {
	key := message.Key()
	if key.Text != "" && key.Mod&^(tea.ModShift|tea.ModCapsLock|tea.ModNumLock) == 0 {
		return true
	}
	switch message.String() {
	case "backspace", "delete", "left", "right", "home", "end", "up", "down",
		"ctrl+a", "ctrl+e", "ctrl+b", "ctrl+f", "ctrl+u", "ctrl+k", "ctrl+w",
		"ctrl+h", "ctrl+d", "ctrl+t", "ctrl+y", "ctrl+_", "ctrl+c",
		"alt+b", "alt+f", "alt+d", "alt+backspace":
		return true
	}
	return false
}

func (h *host) protoPromptMarker(marker promptMarker) error {
	if !h.proto.enabled {
		return nil
	}
	switch marker.kind {
	case 'X':
		h.proto.snapshotReady = true
	case 'C':
		// A command started, so an outstanding snapshot will not be answered
		// at this prompt. Input held behind that Tab is the user's
		// type-ahead for the command: release it in order, as Bash would
		// have received it without the prototype.
		wasTab := h.proto.tabPending
		h.proto.tabPending, h.proto.nativeTabRun = false, false
		h.protoAbandonSeed()
		h.protoDetach()
		if wasTab {
			return h.protoFlushHeldToBash()
		}
	}
	return nil
}

func (h *host) protoTakeCmd() tea.Cmd {
	cmd := h.proto.pendingCmd
	h.proto.pendingCmd = nil
	return cmd
}

// protoAfterAbsorb runs once per absorbed Bash chunk, after the menu-command
// restore path, and acts on a coalesced snapshot acknowledgement.
func (h *host) protoAfterAbsorb() error {
	if !h.proto.enabled || !h.proto.snapshotReady {
		return nil
	}
	h.proto.snapshotReady = false
	wasTab := h.proto.tabPending
	h.proto.tabPending = false
	line, point, err := xtabProtoReadFile(h.child.protoFile(xtabProtoSnapshotName))
	if err != nil {
		// Acked snapshots are complete files, so this is a real failure:
		// stop syncing. A pending Tab still belongs to Bash, as do the keys
		// held behind it.
		h.proto.nativeTabRun = false
		h.protoDetach()
		h.notice = "x-tab prototype: " + err.Error()
		if wasTab {
			return h.protoNativeTab()
		}
		return nil
	}
	prefix, query, cursor, ok := xtabProtoParseLine(line, point)
	if wasTab {
		if !ok {
			return h.protoNativeTab()
		}
		h.proto.nativeTabRun = false
		h.proto.prefix = prefix
		h.proto.pendingCmd = h.protoOpen(query, cursor)
		return nil
	}
	if !ok || !h.protoAttached() {
		h.protoDetach()
		return nil
	}
	h.proto.prefix = prefix
	// A fresh pane that has not acked readiness takes the newest line with
	// its deferred seed; triggering it now could be flushed by its line
	// discipline.
	if seed := h.proto.pendingSeed; seed != nil && seed.pane == h.pane {
		seed.query, seed.cursor = query, cursor
		return nil
	}
	h.protoSeedMenu(h.pane, query, cursor)
	return nil
}
