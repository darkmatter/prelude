package menu

// PROTOTYPE (x <Tab> synced menu filter), menu half. Inert unless the
// process environment carries exactly PRELUDE_X_TAB_PROTOTYPE=1 plus the two
// pane-owned data files the workspace host passes to embedded menu panes
// (PRELUDE_XTAB_PROTO_IN / PRELUDE_XTAB_PROTO_OUT). Files hold
// "<rune caret>\n<root filter query>" as data; acknowledgements are OSC 997
// sequences emitted with tea.Raw into this program's own output, mirroring
// the host's OSC 133 pattern. The host seeds this filter by replacing the
// in-file and sending one 0x1e (ctrl+^) trigger byte; Init announces
// readiness first, so a freshly started process never depends on input sent
// before Bubble Tea entered raw mode. Files are replaced by rename, so
// neither side reads a half-written one. Only the root list filter syncs:
// subcommand pickers and argument entry keep their existing semantics.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

const (
	xtabProtoOutOSC   = "\x1b]997;prelude-xtab-proto-out\a"
	xtabProtoReadyOSC = "\x1b]997;prelude-xtab-proto-ready\a"
	xtabProtoMaxData  = 64 << 10
)

// lastSent/lastCursor are the publish baseline: the root filter text and
// rune caret the host already knows. It starts at the empty prompt a new
// picker shows, so nothing is published before the user edits.
type xtabProto struct {
	inFile     string
	outFile    string
	lastSent   string
	lastCursor int
}

func newXtabProtoFromEnv() *xtabProto {
	if os.Getenv("PRELUDE_X_TAB_PROTOTYPE") != "1" {
		return nil
	}
	in, out := os.Getenv("PRELUDE_XTAB_PROTO_IN"), os.Getenv("PRELUDE_XTAB_PROTO_OUT")
	if in == "" || out == "" {
		return nil
	}
	return &xtabProto{inFile: in, outFile: out}
}

// protoCursor is the prompt caret in runes.
func (p Prompt) protoCursor() int { return p.input.Position() }

func (p Prompt) protoWithValueCursor(s string, cursor int) Prompt {
	p.input.SetValue(s)
	p.input.SetCursor(cursor)
	return p
}

func (m *model) protoApplySeed(query string, cursor int) {
	m.expanded = false
	m.prompt = m.prompt.protoWithValueCursor(query, cursor)
	m.filter()
	m.syncList()
	// Baseline: host-originated text is never echoed back as a menu edit.
	m.xtab.lastSent, m.xtab.lastCursor = m.prompt.Value(), m.prompt.protoCursor()
}

func (m model) protoInit() tea.Cmd {
	if m.xtab == nil {
		return m.prompt.Init()
	}
	// By the time Init commands run the program is reading raw input, so the
	// host may safely send its first seed trigger after this ack.
	return tea.Batch(m.prompt.Init(), tea.Raw(xtabProtoReadyOSC))
}

func xtabProtoTriggerKey(msg tea.KeyPressMsg) bool {
	key := msg.Key()
	return key.Mod&tea.ModCtrl != 0 && key.Code == '^'
}

// protoHandleTrigger consumes the host's reseed byte. It only ever applies
// to the root list filter; inside a subcommand picker or argument entry the
// trigger is still consumed so it cannot leak into the prompt.
func (m model) protoHandleTrigger() (model, tea.Cmd) {
	if m.mode != modeList || m.sub() != nil {
		return m, nil
	}
	if query, cursor, ok := xtabProtoRead(m.xtab.inFile); ok {
		m.protoApplySeed(query, cursor)
	}
	return m, nil
}

// protoPublish writes the root filter to the out-file and acks when local
// editing changed its text or caret since the last sync. Mode/picker
// transitions only re-baseline: restored or cleared prompts from entering
// or leaving subcommand/argument modes are navigation, not filter edits.
func (m model) protoPublish(prevMode mode, prevPickers int, cmd tea.Cmd) (model, tea.Cmd) {
	if m.xtab == nil {
		return m, cmd
	}
	value, cursor := m.prompt.Value(), m.prompt.protoCursor()
	if m.mode != modeList || m.sub() != nil || m.mode != prevMode || len(m.pickers) != prevPickers {
		m.xtab.lastSent, m.xtab.lastCursor = value, cursor
		return m, cmd
	}
	if value == m.xtab.lastSent && cursor == m.xtab.lastCursor {
		return m, cmd
	}
	m.xtab.lastSent, m.xtab.lastCursor = value, cursor
	data := []byte(strconv.Itoa(cursor) + "\n" + value)
	if err := xtabProtoWrite(m.xtab.outFile, data); err != nil {
		m.xtab = nil // a broken prototype channel must not break the picker
		return m, cmd
	}
	if cmd == nil {
		return m, tea.Raw(xtabProtoOutOSC)
	}
	return m, tea.Batch(cmd, tea.Raw(xtabProtoOutOSC))
}

func xtabProtoRead(path string) (string, int, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, false
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, xtabProtoMaxData+1))
	if err != nil || len(data) == 0 || len(data) > xtabProtoMaxData {
		return "", 0, false
	}
	head, query, found := strings.Cut(string(data), "\n")
	if !found || !utf8.ValidString(query) || strings.ContainsAny(query, "\r\n\x00") {
		return "", 0, false
	}
	cursor, err := strconv.Atoi(head)
	if err != nil || cursor < 0 || cursor > utf8.RuneCountInString(query) {
		return "", 0, false
	}
	return query, cursor, true
}

// xtabProtoWrite replaces path atomically: the host reads on the ack that
// follows and must see this complete file, never a truncated one.
func xtabProtoWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("write prototype data: %w", err)
	}
	_, writeErr := f.Write(data)
	err = errors.Join(writeErr, f.Close())
	if err == nil {
		err = os.Rename(f.Name(), path)
	}
	if err != nil {
		if removeErr := os.Remove(f.Name()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
		return fmt.Errorf("write prototype data: %w", err)
	}
	return nil
}
