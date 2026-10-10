# Prelude workspace

A real interactive Bash and movable Prelude docs/menu window rendered
inside the current terminal through libghostty-vt's C API and Prelude's existing
Bubble Tea/Ultraviolet stack. Consumer integration is opt-in through
`prelude.workspace.enable`, which defaults to `false`. When enabled, generated
project init enters `prelude-workspace` in the foreground in interactive Bash
with a TTY. Manual launches remain available.

The workspace uses a separate Go module at
[`src/cmd/prelude-workspace/go.mod`](../../src/cmd/prelude-workspace/go.mod)
so its native renderer dependency pin stays intentionally isolated. It pins
upstream Ultraviolet fixes for decomposed combining characters and wide-cell
repainting when moving or resizing panes. Normal Prelude surfaces keep their
existing dependencies.

## Enable in consumers

Both `flakeModules.default` and `lib.evalModule` support the same opt-in setting.
Add it to the `prelude.nix` sidecar used by either install path, alongside the
required surfaces (keep your existing Docs pages if you already have them):

```nix
{
  prelude = {
    workspace.enable = true;
    menu.enable = true;
    motd.enable = true;
    docs.pages = [ { text = ./README.md; } ];
  };
}
```

Enabling the workspace does not implicitly enable its surfaces. Evaluation
assertions require `prelude.menu.enable = true`, `prelude.motd.enable = true`,
and non-empty `prelude.docs.pages`; failures identify the missing prerequisite
and advise enabling it or disabling the workspace. `prelude.prompt.enable` is
not required. Automatic workspace entry uses Starship when that option is `true`
and the fixed `prelude $ ` prompt otherwise. Manual `--starship` launches use the
consumer's prompt settings even when `prelude.prompt.enable = false`.

When enabled, both entrypoints export `packages.prelude-workspace` and bundle the
active launcher in `prelude-shell`. Keep using `prelude-shell` in your devshell
as described in [Install](../../README.md#install); no workspace shellHook is
needed. The launcher binds Menu, Docs, MOTD, theme, and completion to that
consumer's evaluated config, not this repository's config.

## Run

Keep `.envrc` as the conventional loader; do not put a workspace launch in it:

```sh
use flake
```

Keep the existing hooks in your interactive Bash rc in this order:

```sh
eval "$(direnv hook bash)"
eval "$(prelude hook bash)"
```

The Prelude hook sources the generated project init (`PRELUDE_INIT`) after
direnv has loaded the environment. That init enters the enabled workspace in
the foreground; `nix develop` also sources it in its interactive Bash. It never
auto-launches from a noninteractive shell, `.envrc` evaluation, lorri, zsh, or
without a TTY. `PRELUDE_WORKSPACE_ACTIVE` prevents workspace children from
recursively entering another workspace. Workspace mode skips legacy BLE
(ble.sh) initialization.

Each generated init is stamped **before** attempting the launch. Exiting the
workspace, or a failed launch, returns to the outer shell without immediately
reopening it at the next prompt. Leaving and reentering the project environment,
or receiving a changed init, permits automatic entry again.

Automatic entry follows `prelude.prompt.enable` for Starship. You can still
launch manually, including reopening after exit or failure:

```sh
x prelude:workspace             # fixed prelude $ prompt
prelude-workspace --starship    # Starship even when prelude.prompt.enable = false
```

Both commands use the current checkout's menu through the active launcher in
`prelude-shell`, rather than a menu bound to a published source snapshot.

The launcher supplies Starship and derives its prompt from the consumer's
evaluated theme and settings, even when `prelude.prompt.enable = false`.
Its original bracketed right-side keymap reads
**Alt + [m] motd · [x] menu · [d] docs**. These are direct Alt shortcuts, not a
prefix sequence. An inherited normal Prelude config selects this workspace
preset; an explicit custom `STARSHIP_CONFIG` is preserved. Outside workspace
mode, Prelude activation and its `?`/`x`/`d` shortcuts are unchanged.

### Published package

From a flake that exports the enabled workspace package (including this
repository), launch it directly; the `path:` form also includes unstaged new files:

```sh
nix run path:.#prelude-workspace
nix run path:.#prelude-workspace -- --starship
```

Unlike the devshell launcher, the published package honors `prelude.root` and
uses the consumer's published Menu. `flakeModules.default` sets that root to
the flake's own source. With `lib.evalModule`, set it explicitly to your flake's
`self` when evaluating the sidecar. For example, in your flake's `outputs`, with
`self`, `inputs`, and the target system's `pkgs` in scope:

```nix
evaluated = inputs.prelude.lib.evalModule pkgs {
  imports = [ ./prelude.nix ];
  prelude.root = self;
};
```

Use `evaluated.packages.prelude-shell` in the devshell and expose
`evaluated.packages.prelude-workspace` as your flake's `prelude-workspace` package
to publish it. A remote launch such as
`nix run github:org/repo#prelude-workspace` uses that consumer's published menu,
Docs, MOTD, theme, and completion config. An explicit `PRELUDE_ROOT` still
overrides the root; otherwise the published menu runs against `prelude.root`,
not the caller's checkout.

## Controls

**Alt+M** reprints MOTD in the main shell, **Alt+X** toggles the command menu,
and **Alt+D** toggles docs. The Starship prompt retains the navigation keymap.

**Ctrl+P** unlocks one Prelude command: press it, release it, then press a letter
below. The separate footer highlights **UNLOCKED** while awaiting that command,
then returns to **LOCKED**. Esc or Ctrl+P again locks without an action. This is
a one-command mode, so menu filter text and shell input remain ordinary typing.

| Chord | Action |
| -------------------- | ----------------------------------------------------------- |
| **Alt+M / Ctrl+P m** | Reprint MOTD in the main shell, preserving parked input |
| **Alt+X / Ctrl+P x** | Show/hide the real `x` command menu |
| **Alt+D** | Show/hide the real `docs` viewer |
| **Ctrl+P t** | Show/hide the current window; open the menu if none exists |
| **Ctrl+P d** | Show/hide the real `docs` viewer |
| **Ctrl+P v** | Cycle to the next layout (same as Ctrl+\]) |
| **Ctrl+P Tab** | Switch input focus between Bash and a live interactive pane |
| **Ctrl+P c** | Close the pane/process, rather than just hiding it |
| **Ctrl+P Esc** | Cancel the pending leader |
| **Ctrl+P Ctrl+P** | Lock without an action |

**Ctrl+\]** moves to the next layout and **Ctrl+\[** to the previous one, without
a leader. The cycle is **floating → left → right → top → bottom → floating**;
the four split positions describe the Prelude pane, with Bash on the other side.

Ctrl+\[ requires enhanced keyboard input: legacy terminals send the same byte
for Ctrl+\[ and Esc. Bubble Tea requests key disambiguation automatically on
supporting terminals; otherwise use `Ctrl+\` for previous layout. Ordinary
Esc still belongs to the focused child (or cancels a pending leader).

Ctrl+G is never a Prelude binding, avoiding conflicts with Zellij. It and
ordinary unprefixed `m`, `x`, `d`, `?`, Tab, and Ctrl+C belong to the focused child.
There is
no leader timeout; an unknown chord is consumed and shows a hint. **Ctrl+V**
quotes the next key, bypassing all host bindings, including layout switching.

The window starts hidden, with **floating** as its default placement. Hiding
returns focus to Bash; showing a live window focuses it. `exit` or **Ctrl+D**
at the main Bash prompt leaves the workspace; typing Ctrl+C does not quit the host.
External SIGHUP, SIGTERM, and SIGINT request orderly shutdown with exit codes
129, 143, and 130 respectively: Bubble Tea restores the outer terminal before
existing child/native cleanup removes the private files. During synchronous
initialization, shutdown waits until initialization returns; SIGKILL cannot be
handled.

## Pane behavior

- Only docs and the command menu use the window. **MOTD prints once on entry,
  before the first prompt**, as normal main-shell output in both fixed-prompt
  and `--starship` modes. Startup invokes normal `motd` (not `--pure`), letting
  Preflight update Cache as needed before Render. A failed or missing command
  prints a warning but does not prevent the shell from starting. Commands,
  redraws, resizes, and pane actions do not replay it; run `motd` manually to
  reprint it. Alt+M (or unlocked `m`) also reprints it through the private main-shell
  Readline handoff: an open pane is hidden but retained, the parked line/point/mark
  return afterward, and a busy/PS2 shell queues it until the next primary prompt.
  An occupied pending slot is never replaced. There is no MOTD window or Ctrl+P ?
  chord.

- One Prelude pane exists at a time. Opening menu/docs focuses it. When mouse
  reporting is active, clicks in the shell or an interactive pane also move
  focus.

- Each pane has its own PTY and Ghostty engine. Moving/resizing reflows the
  terminal and delivers SIGWINCH without restarting the child. Placement is
  remembered even when no pane is open. The child fills the pane: there is no
  host border, title strip, or extra inset; placement/focus/status live in the
  protected footer.

- The host adds no pane background or default foreground. The command menu runs
  as `x --embedded --select-output <private file>`: only the outer terminal
  canvas and terminal-wide script preview omit their background fills. The menu panel retains its body, filter,
  title/status chrome, details/form backgrounds, selection bars, and option-chip
  highlights. Standalone `x` and the docs viewer keep their existing styling.
  Unstyled cells inherit the outer terminal's colors; blank cells still mask
  underlying shell text rather than showing two overlapping text layers.
  Hiding restores the live shell underneath.

- Ctrl+P t or repeating the current running surface's Alt shortcut shows/hides it without
  restarting its process. Hidden panes keep their terminal, navigation/filter
  state, and output reader; they continue receiving output but no keyboard input.
  Bash uses the full available area while the window is hidden.

- Choosing a different surface replaces the current child. Ctrl+P c explicitly
  closes it. Placement is remembered across hiding, switching, and closing.

- An exited docs pane or failed pane keeps its final image/status and returns
  focus to Bash. Ctrl+P t shows/hides that image; repeating its surface shortcut
  starts a fresh child. A successful menu selection or cancellation closes the
  picker instead of leaving a dead menu image.

- **Menu selections run in the main Bash**, not the menu pane. The picker
  returns complete shell source through a private file and never executes it.
  The host closes the picker and uses a private readline binding to submit a
  source-file command, keeping Bash's job control, exit status, and Starship
  timing. The original readline line, cursor point, and mark are saved by Bash
  and restored after the selected command finishes; raw script bytes are not
  pasted into terminal input. Task directories and PATH prefixes stay scoped
  to the selected command. Standalone `x` retains its normal execution behavior.

- With the normal Bash prompt lifecycle, a selection made during a foreground
  program or secondary PS2 prompt waits for the next primary prompt instead of
  being sent to that program or spliced into the unfinished command. Opening
  another menu while a selection is waiting shows a hint instead of replacing it.
  This gate trusts the main PTY's OSC133 markers; see the trust limitation below.

- Tiny windows hide the pane and suspend its input/resizing, retaining the
  child and terminal state. Focus returns to Bash and stays there when the
  pane becomes visible again; use Ctrl+P Tab to refocus it.

The cursor starts as a **blinking bar** and Bash restores it whenever a primary
or continuation prompt appears, including with Starship. Child applications can
still change its style or hide it while they are running.

The Starship navigation keymap stays on the prompt. The protected bottom row
is the command-mode footer: **LOCKED · Ctrl+P unlock** normally, and a highlighted
**UNLOCKED** badge followed by available commands after Ctrl+P. It uses the command
menu's padded keycaps, muted labels, and compact spacing. Pane state and notices
align on the right. The unlocked badge is bold with the theme's `success`
background and `bg` foreground; it does not paint the canvas.

Mode hints are the same for generated, fixed, and custom prompts. Selecting a
command relocks automatically, preserving normal shell and menu input. While completion is open,
that same row shows Tab/Shift+Tab/Enter/Esc hints; the chooser adds no second footer.
The diagnostic `prompt | last:… | input:…` bar and its input/exit bookkeeping
are removed. Bash still owns Readline edits, history, and command status.
`clear` cannot erase the host's hints row.

The footer and completion use the menu's resolved palette from
`PRELUDE_MENU_CONFIG`, supplied by the launcher and workspace devshell. Keycaps use
bold `accent2` on `bg`, labels use `muted`, selection uses `accent`, and running
pane status, notices, and failures use `success`/`warning`/`error`. Only the
keycaps paint a background; the canvas and completion candidates remain transparent. Theme and palette overrides in `prelude.nix` therefore apply to
this chrome as well as the menu. The palette is read once at startup; an explicitly
empty `PRELUDE_MENU_CONFIG` inherits terminal-default colors.

Try parking a half-written command in Bash, opening docs with Alt+D, hiding it
with Ctrl+P t,
and finishing the command in Bash. Show docs again to resume where you left off,
then try all five placements in both directions. Hiding, resizing, or moving the
pane should preserve both children and their input state.

## Catalogue completion

The launcher and workspace devshell supply `PRELUDE_COMPLETION_INIT`, a generated
completion-only Bash init. The private rc sources it automatically before the
first prompt in both prompt modes. It loads the canonical command catalogue and
existing completion functions, registering declared argument candidates, direct
commands, and Just completion where enabled. Ordinary file/function and argument
completion remains Bash's responsibility.

At a primary prompt, `x<Tab>` or `x <key-prefix><Tab>` opens a **host-rendered
completion chooser below the prompt** when multiple commands match. **Tab** and
**Shift+Tab** cycle the highlighted item; **Enter** inserts the completion without
executing it, and **Esc** dismisses the chooser without changing the line. A single
match inserts immediately. A later Enter runs the completed command normally in
the main shell. This chooser is separate from the full Alt+X / Ctrl+P x command picker.
Descriptions align in a padded column measured in terminal cells. The chooser
and the single hints footer inherit the terminal background; selection uses the
configured accent and a bold `>` marker rather than a background fill.

Candidates are not printed into Bash's terminal or scrollback. Completion is a
rendering-only overlay: Bash's PTY and Ghostty's native screen keep their full
geometry, with no completion-triggered resize or SIGWINCH. Blank space below the
prompt is used directly; otherwise the host temporarily shifts the displayed
viewport upward, cropping earlier output while preserving the full visible
multiline prompt and wrapped input. The cursor and mouse coordinates follow that
projection. Candidate rows shrink to fit, and the chooser is suppressed when no
safe space remains or the input boundary is unknown. Closing it restores the
unshifted live screen. Existing footer/sidebar geometry stays unchanged.

Bash supplies its actual line, point, and mark through
private, bounded records; accepted edits compare that snapshot before replacing
readline input. Query generations and separate apply acknowledgements reject late
responses. Readline expands real Tab internally, so foreground programs receive
ordinary Tab and PS2 keeps native completion. Editing, paste, focus/layout changes,
or quoted keys dismiss or invalidate the chooser. The popup is keyboard-only.

This completion-only init does not run full `prelude-init`, load ble.sh or the
general bash-completion framework, or reprint MOTD. A missing or failing init
prints a warning but leaves the shell usable.
An unset or empty `PRELUDE_COMPLETION_INIT` skips catalogue initialization; the
launcher respects an explicit override.

The workspace's chooser handles full colon-containing key prefixes, including
`x go:t<Tab>`. Outside the workspace, the completion-only init still provides ordinary
Readline completion; its existing colon-tokenization limitation is unchanged.
The chooser opens on Tab, not automatically on every typed character.

## Starship

Automatic workspace entry uses Starship when `prelude.prompt.enable = true`;
manual `--starship` launches remain available independently of that option.
`--starship` loads the installed full Bash integration, not a simulated prompt.
Bash still owns readline, history, completion, signals, and job control; ble.sh
and user shell rc files are not loaded. The packaged Bash is modern enough for
Starship's PS0 timer (Bash 4.4 or newer).

OSC133 wraps the generated PS1 and is appended to the existing PS0 timer.
Starship's precmd runs first so exit status, pipeline status, and command timing
are preserved. Completion anchors input at OSC133 B, then relocates that boundary
using a bounded rendered prompt-tail fingerprint—not the literal prompt text or
a second input buffer. Only the boundary and prompt height are retained; the host
no longer reconstructs editable input or mirrors Bash's exit status in a footer.
Color and multiline prompts can therefore be used without hardcoding `prelude $ `.

The isolated Ultraviolet pin is `b2b0f8d1567b`: wide/drift-prone rows repaint
from a known cursor position instead of resuming a diff inside a wide glyph's
continuation cell. A footerless renderer replay checks the resulting text,
cell widths, and cursor against an independent terminal emulator, including
CJK, emoji, combining characters, and an ASCII control. Real Starship smoke
coverage requires the full prompt to survive docking, undocking, and closing.

Coverage drives the real Starship binary with both a deterministic colored/wide
multiline config and Prelude's generated Powerline config. It exercises status,
Ctrl+C, timing, editing/history/completion, scroll/resize/redraw, and parked input
behind docs/menu panes. Prelude's ble-specific right prompt and submitted-prompt
rewrite are intentionally not reproduced here.

## Build and verify

The root `flake.lock` pins Nixpkgs' `libghostty-vt` C API (currently
`0.1.0-unstable-2026-05-03`). No Ghostty GUI, Raylib, or new flake input is
needed. The native compile is shared and config-independent; consumer options
change the launcher and bound surface/config artifacts, not the native renderer
build. With `prelude.workspace.enable = false`, the workspace launcher and native
dependencies stay out of consumer shells and closures.

This repository dogfoods `prelude.workspace.enable = true`, so its default
shell includes the active launcher and native runtime dependencies and enters
the workspace in eligible interactive Bash contexts. Native headers,
pkg-config, and renderer tooling remain in the dedicated `workspace` devshell,
not the default shell:

```sh
nix develop path:.#workspace
# In that shell, from the repository root:
go test -C src/cmd/prelude-workspace -race -count=1 ./...
go vet -C src/cmd/prelude-workspace ./...
nix build path:.#checks.x86_64-linux.prelude-workspace
nix build path:.#checks.x86_64-linux.workspace-prompt
```

Use the corresponding system name for the targeted Nix checks. The full
repository gate, `x check path:.`, includes them on the current system. Run checks
on the system you intend to use; these entrypoints do not imply all-platform
verification.

## Intentional limits

- This starts an isolated Bash with a private rc, no user rc/inputrc and no
  persistent history file. Automatic entry uses `prelude.prompt.enable` to choose
  Starship or the fixed `prelude $ ` prompt; manual launches use the fixed prompt
  unless passed `--starship`. It inherits devshell tools and environment
  but isolates inherited shell-hook framework state. OSC133 hooks supply prompt
  lifecycle; this is not arbitrary-shell integration. Surface wrappers inherit
  the host environment directly.

- Completion needs a visible, unambiguous input boundary. Off-screen, empty,
  clipped, or ambiguous right-edge prompt boundaries suppress it until a readable
  redraw. Fingerprints retain at most 256 cells / 4 KiB; they identify prompt
  geometry, not a complete command model.

- One Prelude pane only: no tabs, Process Compose, session daemon, or interactive
  scrollback viewer yet. The native engines retain scrollback for reflow.

- Mode-2027 width negotiation is not synchronized between the native engine and
  the outer renderer. Variation-selector and ZWJ emoji can therefore disagree
  in width. Coverage verifies CJK, single emoji, and composed/decomposed accents,
  not complete outer-terminal capability parity.

- Graphics are disabled and clipboard integration is omitted. The native
  engine's built-in terminfo-query table is not fully filtered for this host.
  Pixel mouse reporting cannot be encoded without pixel geometry; other
  unsupported inputs fail explicitly. Outer-terminal input/display capabilities
  remain the ceiling.

- Shrinking to one row omits the footer; very small windows suspend the pane.

- Queued command handoff uses OSC133 markers from the main PTY as readiness
  signals. They are not authenticated: a foreground program emitting counterfeit
  prompt markers can confuse the gate and receive reserved Readline input
  sequences. A shell-owned readiness protocol needs a separate design before
  claiming unconditional foreground-input isolation.

- Cleanup covers each child's own and current foreground process groups, not
  every job in the Bash session. Ordinary background jobs in separate process
  groups, as well as intentionally detached/disowned descendants, may survive.
  Broader job ownership needs a separate design. Final output drain after a child
  exits is bounded, including when a descendant keeps writing.

## Code map

- [`src/prelude/options/workspace.nix`](../../src/prelude/options/workspace.nix): the opt-in public setting.
- [`src/prelude/packages.nix`](../../src/prelude/packages.nix): prerequisite assertions,
  consumer surface binding, checkout/published launchers, and shell bundling.
- [`src/prelude/workspace.nix`](../../src/prelude/workspace.nix): shared native build,
  launcher, native dependencies, workspace devshell, and checks.
- [`main.go`](../../src/cmd/prelude-workspace/main.go): CLI and outer-terminal lifecycle.
- [`host.go`](../../src/cmd/prelude-workspace/host.go): UI-loop state, prompt tracking, and tagged pane events.
- [`chords.go`](../../src/cmd/prelude-workspace/chords.go): leader, focus, surface switching, and local mouse coordinates.
- [`layout.go`](../../src/cmd/prelude-workspace/layout.go) / [`render.go`](../../src/cmd/prelude-workspace/render.go): pure geometry and shell/pane/footer composition.
- [`pane.go`](../../src/cmd/prelude-workspace/pane.go) / [`process.go`](../../src/cmd/prelude-workspace/process.go): surface wrappers and shared bounded PTY lifecycle.
- [`prompt.go`](../../src/cmd/prelude-workspace/prompt.go): private Bash rc (entry MOTD and prompt hooks), OSC133 parsing,
  and visible-input observation.
- [`selection.go`](../../src/cmd/prelude-workspace/selection.go): picker-result handoff, primary-prompt queueing, and private
  readline bindings that preserve parked input.
- [`completion.go`](../../src/cmd/prelude-workspace/completion.go) / [`completion_bash.go`](../../src/cmd/prelude-workspace/completion_bash.go): host completion choices and the private
  snapshot/edit transport; [`render.go`](../../src/cmd/prelude-workspace/render.go) draws the chooser, not the child terminal.
- [`terminal.go`](../../src/cmd/prelude-workspace/terminal.go) / [`bridge.c`](../../src/cmd/prelude-workspace/bridge.c) / [`bridge.h`](../../src/cmd/prelude-workspace/bridge.h): native engine and Go-owned snapshots.

The C bridge owns native buffers and callbacks; the host event loop exclusively
owns both engines. PTY reader/waiter goroutines never access terminal state.
Native render snapshots are rebuilt to avoid a combining-mark cache bug in the
pinned library. Run editors/language servers in the workspace devshell when working
on its cgo sources; the default devshell intentionally omits native headers.
