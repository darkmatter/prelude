package main

// x-tab prototype, menu pane half: opening and seeding the Alt+W picker,
// holding input for a fresh pane until it acks readiness, and applying the
// picker's filter edits back to Readline. See xtab_protocol.go.

import (
	"strconv"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

type xtabProtoSeed struct {
	pane   *pane
	query  string
	cursor int // runes into query
}

// protoOpen shows the retained menu pane or starts the same picker Alt+W
// starts, focuses it, and seeds it from the snapshot.
func (h *host) protoOpen(query string, cursor int) tea.Cmd {
	if h.surface == surfaceMenu && h.currentPane(h.pane) && h.paneError == nil {
		p := h.pane
		var cmd tea.Cmd
		if !h.paneVisible {
			cmd = h.togglePane()
		} else {
			h.focusPane = h.paneInteractive()
			cmd = h.syncFocus()
		}
		if h.err != nil || p.protoInFile == "" {
			h.protoDetach()
			if err := h.protoFlushHeldToBash(); err != nil {
				return h.fail(err)
			}
			return cmd
		}
		h.proto.active, h.proto.pane = true, p
		h.protoSeedMenu(p, query, cursor) // retained pane is already in raw mode
		if h.proto.active {
			h.protoFlushHeldToPane(p)
		} else if err := h.protoFlushHeldToBash(); err != nil {
			return h.fail(err)
		}
		return cmd
	}
	cmd := h.chooseSurface(surfaceMenu)
	if h.err != nil || !h.currentPane(h.pane) || h.pane.protoInFile == "" {
		h.protoDetach()
		if err := h.protoFlushHeldToBash(); err != nil {
			return h.fail(err)
		}
		return cmd
	}
	h.proto.active, h.proto.pane = true, h.pane
	// The seed (and held input, which stays queued) waits for the fresh
	// menu's ready ack (see protoPaneOutput): its PTY line discipline can
	// still be canonical now and would flush the trigger byte. A menu that
	// never acks must not hold the user's input forever.
	h.proto.pendingSeed = &xtabProtoSeed{pane: h.pane, query: query, cursor: cursor}
	return tea.Batch(cmd, h.protoArmTimeout(xtabProtoReadyTimeout))
}

func (h *host) protoSeedMenu(p *pane, query string, cursor int) {
	err := xtabProtoWriteFile(p.protoInFile, []byte(strconv.Itoa(cursor)+"\n"+query))
	if err == nil {
		err = p.forward([]byte{xtabProtoTrigger}, nil)
	}
	if err != nil {
		h.notice = "x-tab prototype: " + err.Error()
		h.protoDetach()
	}
}

// protoAbandonSeed gives up on seeding a fresh pane: input held for it is
// released to it unseeded, in order, and the session detaches. Without a
// pending seed it does nothing.
func (h *host) protoAbandonSeed() {
	seed := h.proto.pendingSeed
	if seed == nil {
		return
	}
	if h.currentPane(seed.pane) {
		h.protoFlushHeldToPane(seed.pane)
	}
	h.protoDetach()
}

// protoHoldPaneKey queues a menu-bound key while a freshly started pane has
// not acked readiness yet; the seed trigger must reach it first.
func (h *host) protoHoldPaneKey(message tea.KeyPressMsg) bool {
	if !h.protoReady() || h.proto.pendingSeed == nil || h.proto.pendingSeed.pane != h.pane {
		return false
	}
	h.proto.held = append(h.proto.held, xtabProtoHeld{key: message})
	return true
}

func (h *host) protoHoldPanePaste(content string) bool {
	if !h.protoReady() || h.proto.pendingSeed == nil || h.proto.pendingSeed.pane != h.pane {
		return false
	}
	h.proto.held = append(h.proto.held, xtabProtoHeld{paste: content, isPaste: true})
	return true
}

// protoFlushHeldToPane releases held user input to the menu pane in arrival
// order, encoded by the pane terminal. A seed trigger, when one is sent, has
// already gone out when this runs, so it stays first.
func (h *host) protoFlushHeldToPane(p *pane) {
	held := h.proto.held
	h.proto.held = nil
	for _, item := range held {
		var data []byte
		var err error
		if item.isPaste {
			data, err = p.terminal.Paste(item.paste)
		} else {
			data, err = p.terminal.Key(item.key.Key())
		}
		if err == nil {
			err = p.forward(data, nil)
		}
		if err != nil {
			h.notice = "x-tab prototype: " + err.Error()
			h.protoDetach()
			return
		}
	}
}

// protoPaneOutput observes the attached menu's prototype acknowledgements in
// its output stream; all bytes still go to the pane terminal untouched.
func (h *host) protoPaneOutput(p *pane, data []byte) error {
	if !h.protoReady() || p.protoInFile == "" {
		return nil
	}
	if h.proto.readyScan.seen(data) {
		if seed := h.proto.pendingSeed; seed != nil && seed.pane == p {
			h.proto.pendingSeed = nil
			h.protoSeedMenu(p, seed.query, seed.cursor)
			if h.proto.active {
				h.protoFlushHeldToPane(p)
			} else if err := h.protoFlushHeldToBash(); err != nil {
				return err
			}
		}
	}
	if h.proto.outScan.seen(data) && h.proto.active && h.proto.pane == p {
		return h.protoApplyMenu(p)
	}
	return nil
}

func (h *host) protoApplyMenu(p *pane) error {
	query, cursor, err := xtabProtoReadFile(p.protoOutFile)
	if err != nil {
		h.protoDetach()
		h.notice = "x-tab prototype: " + err.Error()
		return nil
	}
	if !xtabProtoValidQuery(query) {
		h.protoDetach()
		return nil
	}
	// A busy or secondary prompt drops this edit instead of queueing a private
	// key for a running program; the next menu edit republishes in full.
	if !h.prompt.ready || h.restoreInput {
		return nil
	}
	prefix := h.proto.prefix
	if prefix == "" {
		prefix = "x "
	}
	line := prefix + query
	point := utf8.RuneCountInString(prefix) + cursor
	data := []byte(strconv.Itoa(point) + "\n" + line)
	if err := xtabProtoWriteFile(h.child.protoFile(xtabProtoApplyName), data); err != nil {
		h.protoDetach()
		h.notice = "x-tab prototype: " + err.Error()
		return nil
	}
	return h.child.send([]byte(xtabProtoApply))
}
