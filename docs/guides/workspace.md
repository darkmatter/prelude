# Prelude workspace

A real interactive Bash and movable Prelude docs/menu window rendered
inside the current terminal through libghostty-vt's C API and Prelude's existing
Bubble Tea/Ultraviolet stack. Consumer integration is controlled through
`prelude.workspace.enable`, which defaults to `true`. When enabled, interactive
`nix develop` sources generated project init and enters `prelude-workspace` in
the foreground in Bash with a TTY. Direnv entry remains lightweight. Manual
launches remain available.

The workspace uses a separate Go module at
[`src/cmd/prelude-workspace/go.mod`](../../src/cmd/prelude-workspace/go.mod)
so its native renderer dependency pin stays intentionally isolated. It pins
upstream Ultraviolet fixes for decomposed combining characters and wide-cell
repainting when moving or resizing panes. Normal Prelude surfaces keep their
existing dependencies.

## Enable in consumers

Both `flakeModules.default` and `lib.evalModule` enable the workspace by default.
Configure the required surfaces in the `prelude.nix` sidecar used by either
install path (keep your existing Docs pages if you already have them):

```nix
{
  prelude = {

    menu.enable = true;
    motd.enable = true;
    docs.pages = [ { text = ./README.md; } ];
  };
}
```

Set `prelude.workspace.enable = false` to opt out. Enabling the workspace does not implicitly enable its surfaces. Evaluation
assertions require `prelude.menu.enable = true`, `prelude.motd.enable = true`,
and non-empty `prelude.docs.pages`; failures identify the missing prerequisite
and advise enabling it or disabling the workspace. `prelude.prompt.enable` is
not required. Automatic workspace entry uses Starship when that option is `true`
and the fixed `prelude $ ` prompt otherwise (passing `--starship=false`). Manual
launches default to Starship and use the consumer's prompt settings even when
`prelude.prompt.enable = false`; `--starship=false` gives the fixed prompt.

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

`cd` + direnv loads the lightweight environment, prints MOTD, and themes an
already-initialized Starship prompt. It does not install the full interactive
footer or catalogue completion. Loader-only MOTD explains that cached entry does
not run `shellHook` in the interactive shell and points to `nix develop` for the
full devshell. The notice is centered within the MOTD width, four blank rows below
the card, with muted prose and only `nix develop` accented. It is hidden in
workspace or quiet mode.

For the full devshell, run interactively:

```sh
nix develop
```

No extra Prelude Bash rc hook is required. `nix develop` sources the generated
project init (`PRELUDE_INIT`) and enters the enabled workspace in the foreground
in interactive Bash with a TTY. The public `prelude hook bash` remains optional:
an already-configured interactive handoff may still use
`eval "$(prelude hook bash)"` after direnv to source that init. It is not an
onboarding requirement. Automatic entry never runs from a noninteractive shell,
`.envrc` evaluation, lorri, zsh, or without a TTY. `PRELUDE_WORKSPACE_ACTIVE`
prevents workspace children from recursively entering another workspace.
Workspace mode skips legacy BLE (ble.sh) initialization.

Each generated init is stamped **before** attempting the launch. Exiting the
workspace, or a failed launch, returns to the outer shell without immediately
reopening it at the next prompt. Leaving and reentering the devshell, or
receiving a changed init, permits automatic entry again through an eligible
interactive init.

Automatic entry follows `prelude.prompt.enable` for Starship. The primary
manual entry is bare `prelude`, which opens the workspace when
`prelude.workspace.enable` is true (the default); `prelude workspace` is the
explicit form and `prelude help` lists subcommands. You can launch manually,
including reopening after exit or failure:

```sh
prelude                         # Starship by default, even when prelude.prompt.enable = false
prelude --starship=false        # fixed prelude $ prompt
```

`x prelude:workspace` and `prelude-workspace` remain equivalent. In a shell
where direnv has loaded the devshell environment (`use flake`), `prelude`
opens the full workspace without `nix develop`, adding only one shell level.
All use the current checkout's menu through the active launcher in
`prelude-shell`, rather than a menu bound to a published source snapshot.

The launcher supplies Starship and derives its prompt from the consumer's
evaluated theme and settings, even when `prelude.prompt.enable = false`.
Its original bracketed right-side keymap reads
**Alt + [m] motd · [x] menu · [d] docs**. These are direct Alt shortcuts, not a
prefix sequence. An inherited normal Prelude config selects this workspace
preset; an explicit custom `STARSHIP_CONFIG` is preserved. Standalone interactive
Prelude activation and its `?`/`x`/`d` shortcuts are unchanged.

### Published package

From a flake that exports the enabled workspace package (including this
repository), launch it directly; the `path:` form also includes unstaged new files:

```sh
nix run path:.#prelude-workspace
nix run path:.#prelude-workspace -- --starship=false
nix run path:.#prelude            # dispatcher form; bare `prelude` opens the workspace
```

For Prelude's own published workspace, `nix run github:darkmatter/prelude`
does the same. For consumers, `nix run .#prelude` opens their published
workspace, the same as `nix run .#prelude-workspace`.

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
**Alt+D** toggles docs, **Alt+W** shows/hides the current window (opening the
menu if none exists), and **Alt+Tab** switches input focus between Bash and a
live interactive window. The Starship prompt retains the navigation keymap.

**Ctrl+P** unlocks layout commands while a window is open; with no window it
reaches Bash as usual. The footer's **^P layout** badge fills with accent and
stays unlocked across these commands, so you can cycle layouts repeatedly.
Esc or Ctrl+P again locks.
While unlocked, other keys are consumed with a hint rather than reaching Bash or
the pane; Alt+M/X/D/W/Tab and Ctrl+G lock before they act.

| Key (unlocked) | Action |
| -------------- | ----------------------------------------------------------- |
| **\]** / **\[** | Move to the next / previous layout |
| **Tab** | Switch focus, for desktops that take Alt+Tab themselves |
| **Esc** | Lock (Ctrl+P also locks) |

**Ctrl+\]** moves to the next layout and **Ctrl+\[** to the previous one without
unlocking. The cycle is **floating → left → right → top → bottom → floating**;
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
  and Starship modes. Startup invokes normal `motd` (not `--pure`), letting
  Preflight update Cache as needed before Render. A failed or missing command
  prints a warning but does not prevent the shell from starting. Commands,
  redraws, resizes, and pane actions do not replay it; run `motd` manually to
  reprint it. Alt+M also reprints it through the private main-shell
  Readline handoff: an open pane is hidden but retained, the parked line/point/mark
  return afterward, and a busy/PS2 shell queues it until the next primary prompt.
  An occupied pending slot is never replaced. There is no MOTD window or Ctrl+P ?
  chord.

- One Prelude pane exists at a time. Opening menu/docs focuses it. Clicks in the
  shell or an interactive pane move focus. The wheel scrolls the band under the
  pointer through its retained history; typing or pasting snaps that band back
  to the live screen. A child that requests mouse tracking receives wheel events
  itself, as in any terminal. A full-screen child that does not track the mouse
  (the menu, `less`, `man`) instead receives the Up/Down key the
  outer terminal used to send. Mouse reporting is always on so the host can claim
  the wheel; use Shift+drag for the outer terminal's own text selection.

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

- Unlocked `t` or repeating the current running surface's Alt shortcut shows/hides it without
  restarting its process. Hidden panes keep their terminal, navigation/filter
  state, and output reader; they continue receiving output but no keyboard input.
  Bash uses the full available area while the window is hidden.

- Choosing a different surface replaces the current child; quitting it (for
  example Esc or `q` in the menu or docs) closes it. Placement is remembered
  across hiding, switching, and closing.

- An exited docs pane or failed pane keeps its final image/status and returns
  focus to Bash. Unlocked `t` shows/hides that image; repeating its surface shortcut
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
  pane becomes visible again; use Tab while unlocked to refocus it.

The cursor starts as a **blinking bar** and Bash restores it whenever a primary
or continuation prompt appears, including with Starship. Child applications can
still change its style or hide it while they are running.

The Starship navigation keymap stays on the prompt. The protected bottom row
is the command-mode footer. Its left side is empty until a window opens; then
a **^P layout** badge joins the key and what it unlocks. After Ctrl+P the badge
fills, followed by `[ ]` labelled with a placement strip `▣ ◧ ◨ ⬒ ⬓`
(floating, left, right, top, bottom; the current one in `accent`) and `Esc lock`. It uses the command
menu's padded keycaps, muted labels, and compact spacing. On terminals at least
8 rows tall, a blank row separates it from Bash and panes; neither can draw
there, and Bash's PTY is sized to stop above it. The unlocked badge is
bold with the theme's `accent` background and `bg` foreground; it does not
paint the canvas.

The right side is an always-on powerline pill after `⌥ +`; each segment names an
Option/Alt key and what it cycles. With no window on screen the pill is a single
dimmed `w viewer` chip with the `w` key in bold `accent2`, like the `^P` keycap. With
a window shown, the `w hide` chip is filled with the theme's `accent` and the pill
adds both input targets, e.g. `w hide ▸ docs ◂ ⇥ shell`: the target receiving
keys is bold `accent` text, the thin muted divider between the two points at
it, and `⇥` marks the one Alt+Tab reaches. Showing a window
focuses it. When the window's process stops, `exited N` or `error` precede the
pill in regular text, and a failure (error or nonzero exit) turns the window's
segment `error` red (the `viewer` label, while hidden). The pill's caps are Nerd Font powerline glyphs, as in the generated prompt.
Without an `accent` color the `w hide` chip is shown in reverse video instead.

Mode hints are the same for generated, fixed, and custom prompts. Command mode
stays unlocked until Esc or Ctrl+P, keeping the strip and pill visible.
The diagnostic `prompt | last:… | input:…` bar and its input/exit bookkeeping
are removed. Bash still owns Readline edits, history, and command status.
`clear` cannot erase the host's hints row.

The footer uses the menu's resolved palette from
`PRELUDE_MENU_CONFIG`, supplied by the launcher and workspace devshell. Keycaps use
bold `accent2` on `bg`, labels and separators use `muted`, status values use
`fg`, and the current layout uses `accent`. Notices and queued commands use
`warning`; failures keep `fg` text and mark themselves with an `error` element
(the pill segment, or a dot after a host error). Only the
keycaps, the unlocked badge, and the viewer pill paint a background; the canvas remains transparent. Theme and palette overrides in `prelude.nix` therefore apply to
this chrome as well as the menu. The palette is read once at startup; an explicitly
empty `PRELUDE_MENU_CONFIG` inherits terminal-default colors.

Try parking a half-written command in Bash, opening docs with Alt+D, hiding it
with Alt+D again,
and finishing the command in Bash. Show docs again to resume where you left off,
then press `]` and `[` while unlocked to try all five placements in both
directions. Hiding, resizing, or moving the
pane should preserve both children and their input state.

## Catalogue completion

The launcher and workspace devshell supply `PRELUDE_COMPLETION_INIT`, a generated
completion-only Bash init. The private rc sources it automatically before the
first prompt in both prompt modes. It loads the canonical command catalogue and
existing completion functions, registering declared argument candidates, direct
commands, and Just completion where enabled.

Tab goes straight to Bash: there is no host completion chooser or overlay.
Readline owns every completion — catalogue keys after `x`, declared argument
candidates, and ordinary file/function completion alike. A single match inserts
immediately; the private rc sets `show-all-if-ambiguous on`, so ambiguous
matches list on the first Tab in Bash's own output. A later Enter runs the
completed command normally in the main shell. The full command picker remains
Alt+X. Foreground programs and PS2 receive ordinary Tab untouched.

This completion-only init does not run full `prelude-init`, load ble.sh or the
general bash-completion framework, or reprint MOTD. A missing or failing init
prints a warning but leaves the shell usable.
An unset or empty `PRELUDE_COMPLETION_INIT` skips catalogue initialization; the
launcher respects an explicit override.

Colon keys complete too, in the workspace and in plain shells using the
completion-only init: `x go:t<Tab>` completes to `x go:test`, and
`x go:<Tab>` lists the matching keys. (Readline splits the current word
at `:`; the completion function rejoins the pieces for matching and
returns each candidate relative to the last `:` so insertion lands
correctly, without changing `COMP_WORDBREAKS`.)

## Starship

Automatic workspace entry uses Starship when `prelude.prompt.enable = true`
and passes `--starship=false` otherwise; manual launches default to Starship
independently of that option, with `--starship=false` for the fixed prompt.
Starship mode loads the installed full Bash integration, not a simulated prompt.
Bash still owns readline, history, completion, signals, and job control; ble.sh
and user shell rc files are not loaded. The packaged Bash is modern enough for
Starship's PS0 timer (Bash 4.4 or newer).

OSC133 wraps the generated PS1 and is appended to the existing PS0 timer.
Starship's precmd runs first so exit status, pipeline status, and command timing
are preserved. The host tracks only the OSC133 prompt lifecycle (and the
editable-input boundary at B) for menu/MOTD handoff; it does not reconstruct
editable input or mirror Bash's exit status in a footer.
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
shell includes the active launcher and native runtime dependencies. Interactive
`nix develop` enters the workspace in Bash with a TTY; direnv entry remains
lightweight. Native headers, pkg-config, and renderer tooling remain in the
dedicated `workspace` devshell, not the default shell:

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
  Starship or the fixed `prelude $ ` prompt; manual launches default to Starship
  unless passed `--starship=false`. It inherits devshell tools and environment
  but isolates inherited shell-hook framework state. OSC133 hooks supply prompt
  lifecycle; this is not arbitrary-shell integration. Surface wrappers inherit
  the host environment directly.

- Catalogue completion is plain Bash Readline completion: candidates are
  Bash's own listing in scrollback, with no chooser, description column,
  or Tab cycling.

- One Prelude pane only: no tabs, Process Compose, session daemon, scrollbar,
  paging keys, or transcript search. The native engines retain scrollback for
  reflow and for wheel scrolling.

- Mode-2027 width negotiation is not synchronized between the native engine and
  the outer renderer. Variation-selector and ZWJ emoji can therefore disagree
  in width. Coverage verifies CJK, single emoji, and composed/decomposed accents,
  not complete outer-terminal capability parity.

- Graphics are disabled. Clipboard sets (OSC 52) from the shell and pane are
  forwarded to the outer terminal (standard, selection, and primary targets);
  clipboard read queries are never answered. The native engine's built-in
  terminfo-query table is not fully filtered for this host.
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

- [`src/prelude/options/workspace.nix`](../../src/prelude/options/workspace.nix): the default-enabled public setting.
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
- [`render.go`](../../src/cmd/prelude-workspace/render.go) draws only the child terminals and footer; completion candidates are Bash's own output, never a host overlay.
- [`terminal.go`](../../src/cmd/prelude-workspace/terminal.go) / [`bridge.c`](../../src/cmd/prelude-workspace/bridge.c) / [`bridge.h`](../../src/cmd/prelude-workspace/bridge.h): native engine and Go-owned snapshots.

The C bridge owns native buffers and callbacks; the host event loop exclusively
owns both engines. PTY reader/waiter goroutines never access terminal state.
Native render snapshots are rebuilt to avoid a combining-mark cache bug in the
pinned library. Run editors/language servers in the workspace devshell when working
on its cgo sources; the default devshell intentionally omits native headers.
