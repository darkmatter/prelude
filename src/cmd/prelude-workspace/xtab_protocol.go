package main

// PROTOTYPE (x <Tab> synced menu filter): compiled in always, active only
// when the workspace process is launched with exactly
// PRELUDE_X_TAB_PROTOTYPE=1. Without it no private files, binds, interceptions
// or pane plumbing exist and Tab keeps Bash's native completion behaviour.
//
// Protocol (files carry data, never shell source; no polling):
//   - Tab at a ready primary prompt is swallowed by the host, which sends the
//     private bind-x key ESC[994~. Bash writes "<character point>\n<line>" to
//     its private snapshot file and acks with OSC 133;X on its PTY output.
//     (READLINE_POINT has counted characters, not bytes, since Bash 5.0.)
//     Bash-bound keys/paste arriving while that snapshot is in flight are
//     held and released after the decision, so input order is preserved.
//     A Tab right after a key that may start a Readline key sequence (a lone
//     Esc, the Ctrl+X prefix) stays native: a private key there would extend
//     the user's sequence instead of running the bind-x.
//   - The host validates the snapshot as a simple top-level `x <token>` line.
//     Eligible: the real Alt+W menu pane opens (or the retained one shows) and
//     is seeded through its pane-owned in-file plus a 0x1e trigger byte; for a
//     freshly started pane the trigger (and any held input) waits for the
//     menu's OSC 997 ready ack so canonical-mode startup cannot drop it.
//     Ineligible or unreadable: one native \t is replayed to Bash so its own
//     completion, inputrc and status apply, and consecutive Tabs then go
//     straight to Bash without another snapshot (a bind-x between Tabs would
//     reset Readline's completion state, breaking e.g. menu-complete cycling).
//   - The menu publishes root-filter edits to its pane-owned out-file and acks
//     with OSC 997 in its PTY output. The host writes "<character point>\n<full
//     line>" to Bash's private apply file and sends ESC[995~ only at a ready
//     prompt; the bind-x function assigns READLINE_LINE/POINT as data.
//   - While a session is attached, simple editing keys forwarded to Bash are
//     followed by ESC[994~ so the open picker re-seeds from the actual
//     Readline buffer. Execution (OSC 133;C), a submit key, an ineligible
//     line, or the pane going away detaches the session; private keys are
//     never sent to a foreground program or a secondary prompt.
//   - Every file is replaced by an atomic rename after a complete write, so
//     a reader never sees a truncated or half-written file. Acks are only
//     sent for writes that succeeded.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	xtabProtoEnv      = "PRELUDE_X_TAB_PROTOTYPE"
	xtabProtoEnvIn    = "PRELUDE_XTAB_PROTO_IN"
	xtabProtoEnvOut   = "PRELUDE_XTAB_PROTO_OUT"
	xtabProtoSnapshot = "\x1b[994~"
	xtabProtoApply    = "\x1b[995~"
	xtabProtoTrigger  = byte(0x1e) // ctrl+^ in the menu's key decoder
	xtabProtoOutOSC   = "\x1b]997;prelude-xtab-proto-out\a"
	xtabProtoReadyOSC = "\x1b]997;prelude-xtab-proto-ready\a"
	xtabProtoMaxData  = 64 << 10
	// One-shot lost-ack escapes (not polling). A Bash snapshot is a builtin
	// write plus one rename; a fresh menu pane must start a whole process.
	xtabProtoTabTimeout   = 500 * time.Millisecond
	xtabProtoReadyTimeout = 2 * time.Second
	xtabProtoDenyRunes    = "'\"\\`$;|&<>(){}"
	// Bash's private files, inside its private (0700) directory.
	xtabProtoSnapshotName = "snapshot"
	xtabProtoApplyName    = "apply"
)

// bashXTabPrototypeRC mirrors the bashSelectionRC pattern: bind-x functions
// read/write private files as data (never source/eval) and preserve $?. The
// snapshot is written with >| (a user's `set -C` must not refuse it) to a
// sibling in the private directory, renamed into place, and acked only if
// both steps succeeded; stderr stays off the prompt line either way.
const bashXTabPrototypeRC = `__prelude_xtab_proto_snapshot() {
    local status=$? file=$__prelude_xtab_proto_dir/` + xtabProtoSnapshotName + `
    if { builtin printf '%s\n%s' "$READLINE_POINT" "$READLINE_LINE" >| "$file.tmp" &&
        builtin command mv -f -- "$file.tmp" "$file"; } 2>/dev/null; then
        builtin printf '\e]133;X\a'
    fi
    return "$status"
}
__prelude_xtab_proto_apply() {
    local status=$?
    local point= line=
    {
        IFS= builtin read -r point
        IFS= builtin read -r line
    } 2>/dev/null < "$__prelude_xtab_proto_dir/` + xtabProtoApplyName + `"
    if [[ $point =~ ^[0-9]+$ ]]; then
        READLINE_LINE=$line
        READLINE_POINT=$point
        READLINE_MARK=0
    fi
    return "$status"
}
builtin bind -x '"\e[994~":__prelude_xtab_proto_snapshot'
builtin bind -x '"\e[995~":__prelude_xtab_proto_apply'
`

// xtabProtoOptIn is true only for an exact "1"; like a process environment,
// the last entry for the name wins.
func xtabProtoOptIn(env []string) bool {
	enabled := false
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == xtabProtoEnv {
			enabled = value == "1"
		}
	}
	return enabled
}

// xtabProtoNeedle is a tiny stateful matcher for one fixed byte sequence in a
// PTY stream whose chunk boundaries may split the sequence. The sequences it
// matches start with ESC and contain no other ESC, so restarting a mismatch
// at seq[0] cannot miss an overlapping match.
type xtabProtoNeedle struct {
	seq  string
	prog int
}

func (n *xtabProtoNeedle) seen(data []byte) bool {
	found := false
	for _, b := range data {
		if b == n.seq[n.prog] {
			n.prog++
			if n.prog == len(n.seq) {
				found, n.prog = true, 0
			}
		} else if b == n.seq[0] {
			n.prog = 1
		} else {
			n.prog = 0
		}
	}
	return found
}

// xtabProtoReadFile parses "<number>\n<rest>"; a missing "\n" means an empty
// rest. The number is a caret in characters (runes) in both directions:
// Bash's READLINE_POINT (characters since Bash 5.0) for snapshots, the menu
// prompt caret for menu files.
func xtabProtoReadFile(path string) (string, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("read prototype data: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, xtabProtoMaxData+1))
	if err != nil {
		return "", 0, fmt.Errorf("read prototype data: %w", err)
	}
	if len(data) > xtabProtoMaxData || strings.IndexByte(string(data), 0) >= 0 {
		return "", 0, fmt.Errorf("prototype data exceeds limit or contains NUL")
	}
	head, rest, _ := strings.Cut(string(data), "\n")
	n, err := strconv.Atoi(head)
	if err != nil || n < 0 {
		return "", 0, fmt.Errorf("prototype data has invalid caret %q", head)
	}
	if !utf8.ValidString(rest) {
		return "", 0, fmt.Errorf("prototype data is not UTF-8")
	}
	return rest, n, nil
}

// xtabProtoWriteFile replaces path atomically: readers acting on an ack see
// either the previous complete file or this one, never a truncated one.
func xtabProtoWriteFile(path string, data []byte) error {
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

// xtabProtoParseLine accepts only "[spaces]x <single simple token>" with the
// caret at or after the prefix. It preserves the user's exact prefix.
// pointChars is READLINE_POINT: a character index, not a byte offset.
func xtabProtoParseLine(line string, pointChars int) (prefix, query string, cursorRunes int, ok bool) {
	i := 0
	for i < len(line) && line[i] == ' ' {
		i++
	}
	if !strings.HasPrefix(line[i:], "x ") {
		return "", "", 0, false
	}
	prefix = line[:i+2]
	// The prefix is spaces and "x " only, so its byte and character
	// lengths agree; the whole-line bound needs the character count.
	if pointChars < len(prefix) || pointChars > utf8.RuneCountInString(line) {
		return "", "", 0, false
	}
	query = line[len(prefix):]
	if !xtabProtoValidQuery(query) {
		return "", "", 0, false
	}
	return prefix, query, pointChars - len(prefix), true
}

func xtabProtoValidQuery(query string) bool {
	for _, r := range query {
		if unicode.IsSpace(r) || unicode.IsControl(r) ||
			strings.ContainsRune(xtabProtoDenyRunes, r) {
			return false
		}
	}
	return true
}

// protoPaneEnv creates the pane-owned sync files and points the menu process
// at them. Without the opt-in it returns env untouched. Both sides replace
// these files through O_EXCL temporaries and rename, so the shared temp
// directory is safe for them.
func (p *pane) protoPaneEnv(env []string) ([]string, error) {
	if !xtabProtoOptIn(env) {
		return env, nil
	}
	for _, slot := range []*string{&p.protoInFile, &p.protoOutFile} {
		f, err := os.CreateTemp("", "prelude-xtab-proto-*")
		if err != nil {
			return env, fmt.Errorf("private x-tab prototype file: %w", err)
		}
		*slot = f.Name()
		if err := f.Close(); err != nil {
			return env, err
		}
	}
	out := append([]string(nil), env...)
	return append(out,
		xtabProtoEnvIn+"="+p.protoInFile,
		xtabProtoEnvOut+"="+p.protoOutFile), nil
}

// xtabProtoBashDir creates the main Bash's private (0700) directory for its
// snapshot/apply files. Bash writes its temporary snapshot by a fixed name,
// which is only safe where no other user can plant a symlink. Without the
// opt-in it returns "" and no prototype RC is installed.
func xtabProtoBashDir(env []string) (string, error) {
	if !xtabProtoOptIn(env) {
		return "", nil
	}
	dir, err := os.MkdirTemp("", "prelude-xtab-proto-bash-*")
	if err != nil {
		return "", fmt.Errorf("private x-tab prototype Bash directory: %w", err)
	}
	return dir, nil
}

func (s *shellProcess) protoFile(name string) string {
	return filepath.Join(s.protoDir, name)
}
