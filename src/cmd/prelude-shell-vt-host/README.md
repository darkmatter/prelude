# prelude-shell-vt-host

A maintainable terminal host for composing Prelude surfaces with an
interactive shell: the invariants are pinned down by tests, and the shutdown,
rendering, and input paths are correct rather than merely demonstrable.

## The idea

A real interactive shell runs in a short pane while Prelude owns pinned rows
above it and a status row below. The child never writes to the outer terminal.
It talks to an in-process virtual terminal, so `CUP`, `ED`, `DECSTBM`, saved
cursors, and the alternate screen all resolve inside a private screen; only the
resulting cells are composed into the frame.

Nothing rewrites the child's escape sequences into outer coordinates. That is
what makes the pinned rows structurally safe rather than defended — there is no
sequence the child can emit that addresses a row it does not own.

```
┌──────────────────────────┐  pin band    captured through its own emulator
│ MOTD / x / Docs          │
├──────────────────────────┤  shell band  child's virtual screen + scrollback
│ $ …                      │
├──────────────────────────┤  status row  host state
└──────────────────────────┘
```

Everything on screen is a `surface` or a live child. A captured panel and a
placeholder are cell images; the shell and a live pinned pane are running
programs drawn straight out of their own virtual terminals. Composition is a
blit either way, and nothing needs coordinate translation.

## Pinned surfaces: captured vs live

A pin mode is one of two things, and the difference decides everything else
about it.

**Captured** (MOTD, x) runs a command once, photographs the terminal it
painted, kills it, and keeps the cells. Right for a program that produces a
picture and exits.

**Live** (Docs) starts a child and leaves it running in the band. Docs is an
interactive TUI, so a snapshot of its first frame is a picture of a program
nobody can use — you would see the docs but never turn a page. A live pane is
a full `shell`: same drawing, resizing, scrollback, and shutdown as the child
shell, differing only in which band it occupies and that its band is chrome
and therefore composed opaquely.

Because a live pane takes input, the host needs a notion of **focus**. Only one
child is active; the shell holds it by default. Pinning does not steal it — you
pass through Docs on the way back to off, and having the shell go deaf
mid-cycle would be hostile — so focus is asked for explicitly with Ctrl+O.

Focus owns *every* input modality, not just keystrokes:

| Signal | Goes to |
| --- | --- |
| keys, paste | the focused child |
| terminal focus/blur reports | the focused child only, and only while the window itself is focused |
| the cursor | the focused child's band |
| Shift+PgUp/PgDn | the shell's scrollback while shell-focused; the pane's own to interpret otherwise |
| mouse events | the child whose **band contains the pointer**, regardless of focus |
| mouse tracking mode | enabled if *either* child asks |

Mouse is the deliberate exception: position is its own targeting, so a wheel
over the pinned pane scrolls the pane even while the shell owns the keyboard,
the way every other split-pane terminal behaves. Dispatching by band is safe
because a child that never requested tracking has its emulator drop the event.

State holds the invariant that only a live pane with rows on screen can take
focus, so unpinning, cycling to a captured mode, or shrinking the terminal past
the pane all hand input back to the shell rather than routing it at something
gone.

## Run

```bash
cd src/cmd/prelude-shell-vt-host && go run .
```

`-shell-rows 1` makes the isolation obvious; the child still behaves normally
in a single row.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-shell-rows` | `10` | shell rows while a surface is pinned |
| `-shell` | `$SHELL` | child command |
| `-docs` | `docs` | program a live Docs pin runs |
| `-scrollback` | `5000` | retained child lines |
| `-fps` | `60` | frame ceiling while the child is chatty |

| Key | Action |
| --- | --- |
| **Ctrl+G** | cycle pin: off → MOTD → x → Docs → off |
| **Ctrl+O** | move input between the shell and a live pane |
| **Shift+PgUp / PgDn** | scroll the child's history |
| `exit` / **Ctrl+D** | leave; the host exits with the child's status |

## Design corrections

**Rendering.** One reused frame buffer instead of a fresh allocation per paint,
and cells stored by value in a single slice per surface. Composition steps over
wide graphemes by cell width; writing every column made ultraviolet blank each
wide glyph as the following continuation cell landed.

**Output.** A reader goroutine feeds the virtual terminal and a throttle
coalesces repaints: the first byte after an idle period paints immediately, and
a burst collapses to one trailing frame. Issuing one `tea.Cmd` per read meant a
chatty child scheduled a frame per chunk.

**Resize.** Applied only when the geometry actually changes, so dragging a
window edge does not become a `SIGWINCH` storm, and a resize that leaves the
pin band alone no longer discards a good capture.

**Mouse.** Mirrored from the child's DEC private modes rather than grabbed
unconditionally, so native selection keeps working whenever the child is not
asking for the mouse.

**Exit status.** Propagated from the child, with host failures distinguished
from child failures via `hostError`.

**Panels.** Captured through their own virtual terminal at exactly the pin
body's geometry. A command that repaints itself lands as the picture it meant
to draw rather than the transcript of how it got there — using `ansi.Strip`
kept erased text and dropped colour. A partial image survives a
nonzero exit, so a failing command still shows the operator something real.

**Opacity is a per-band policy, not a global one.** A virtual terminal reports
unset colours as nil, meaning "the renderer's default", and hands back a blank
cell rather than nothing for every coordinate the child never touched. What to
do with those nils depends entirely on which band the cells land in, so both
composition paths take a `paint`:

- The **shell band** is transparent. A child using the terminal's own colours
  must reach the frame with them still unset, or the host would force its
  palette on someone running a light theme.
- The **pin band** is opaque. It is chrome sitting on top of the shell, so a
  nil there would resolve against the outer terminal and let the user's theme
  show through it.

Getting this wrong is easy in exactly one direction: a live pane is drawn by
the same `shell.draw` as the shell itself, so reusing the shell's policy for it
would silently reintroduce transparent chrome. That is why the policy is a
parameter rather than a property of the capture path.

**Header.** The pin header is drawn after the panel blit. Drawing the panel
could otherwise overwrite its own header.

## Shutdown

`Emulator.Read` parks until a reply exists and checks a `closed` flag that
upstream does not synchronise against `Close`. Both the shell and panel capture
therefore close the PTY, poke a reply through to unpark the reply pump, join
it, and only then close the emulator.

The poke runs on its own goroutine: the reply channel is an unbuffered pipe, so
once the pump has returned — which it may have done already, on an earlier
failed write — an inline poke would block forever. `retireReplyPump` reports
whether the pump actually came back; a pump that never returns leaves the
emulator open, because leaking a goroutine beats writing the flag out from
under a live reader. `TestStopReturnsWhenTheReplyPumpDiedFirst` covers exactly
that deadlock, and hangs indefinitely if the poke is made synchronous again.

## Tests

80 tests, green under `-race`.

```bash
cd src/cmd/prelude-shell-vt-host && go test -race ./...
```

| File | Covers |
| --- | --- |
| `layout_test.go` | bands tile the frame exactly at every size |
| `state_test.go` | capture generations, stale results, resize dedupe, scroll clamping, focus invariant, live vs captured modes |
| `surface_test.go` | capture keeps the child's colour; opaque and transparent bands; blit offsets, clips, truncates |
| `wide_test.go` | CJK and emoji survive both composition paths; per-band opacity through a live child |
| `throttle_test.go` | leading frame is immediate, bursts coalesce, stop is idempotent |
| `shell_test.go` | live child: output, screen clear vs pinned rows, alt screen, resize, exit status, scrollback |
| `shutdown_test.go` | shutdown terminates in every reply-pump state |
| `panel_test.go` | capture gives a real TTY and geometry, keeps styling, stays opaque end to end, keeps the picture not the transcript, rejects live modes |
| `routing_test.go` | input ownership across the focus switch: keys, paste, focus reports, mouse dispatch, mouse-mode union, scroll keys; pane lifecycle against shrink/grow and stale exits |
| `host_smoke_test.go` | the built binary on a PTY: pinned rows survive a child clear, a pinned pane is driven by the keyboard, focus hands back, exit status propagates |

Three harness notes worth keeping:

- The smoke harness must answer the host's capability queries. Bubble Tea
  probes the terminal at startup and waits for replies before painting, so a
  harness that only records output sees a blank screen forever.
- `startTestShell` sets `LANG`. `/bin/sh` is bash, and readline in the C locale
  silently drops 8-bit input, so the wide-grapheme tests would otherwise be
  measuring the wrong thing. Host input forwarding is byte-exact.
- The pane tests run a scripted stub via `-docs`, not the real viewer, so they
  assert routing rather than whatever the docs binary happens to render today.

## Pane lifecycle

A pane exit is not self-describing. Every pane the host retires on purpose —
because the terminal shrank past it, or because it is being replaced — emits
the same signal as a user quitting the program, and it arrives after the fact.

So `paneGoneMsg` names its child, the host detaches the pane before stopping
it, and the handler ignores any message that is not from the pane currently on
screen. Without that, shrinking the terminal would unpin Docs (so growing back
would come up empty), and a late exit could tear down the pane that had since
replaced it. Both are covered by regressions that fail loudly if the identity
check or the detach ordering is removed.

## Status

The architecture is now complete: the Docs pin is a live pane rather than a
placeholder. The real docs TUI runs in the band, and Ctrl+O hands it the
keyboard so its pages can actually be turned.

Still open: the pin is cycled with Ctrl+G rather than driven by a command the
child shell can run, and MOTD and x remain captured surfaces — correctly, in
their case, since both paint once and exit.
