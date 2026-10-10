# Commands

Prelude supplies these whenever the components are enabled:

- **`x`** (`m`) — public catalogue entrypoint. `x` alone opens the interactive
  picker; `x <key>` runs a catalogue command (e.g. `x go:test`, `x docs`).
  `x --list` prints the table non-interactively. `x --embedded` opens the same
  picker without outer canvas background fills, preserving its themed panel
  backgrounds and selection highlights;
  place this render-mode flag before a command key. `x --select-output <path>`
  returns complete shell source to a private file without executing it (empty
  on cancellation); hosts can combine it with `--embedded`.
- **`motd`** (`?`) — reprints the welcome banner.
- **`docs`** (`d`) — this viewer (`x docs`).
- **`portal`** (`p`) — app launcher with live health lights.

For catalogue-only Bash completion, source `"$PRELUDE_COMPLETION_INIT"` after
loading the devshell environment. This registers command and argument completion
without loading ble.sh, initializing Starship, printing MOTD, or changing the
status row. The Ghostty spike does this automatically; normal activation is
unchanged.

Project commands declared in `nix/internal/prelude.nix`:

- **`x go:test`**, **`x go:vet`** — public catalogue commands grouped under
  `go`; they dispatch to canonical `go test -C src ./...` / `go vet -C src ./...` without generating duplicate executables.
- **`x check`** — `nix flake check`: builds every package and render check.
- **`x fmt`** — `treefmt` (alejandra for Nix, gofmt/goimports for Go) over the repository.
- **`x build <target>`** — `nix build` with flake-output suggestions.
- **`x prelude:previews`** — build the render checks and display their output.
- **`x spike:ghostty`** — launch the isolated libghostty-vt shell experiment:
  a single themed hints footer and borderless, floating-by-default docs/menu
  window, with no host-imposed backdrop. Menu panes use `x --embedded` automatically.
  Alt+M reprints MOTD in the main shell, Alt+X toggles the menu, and Alt+D toggles
  docs. Ctrl+P unlocks one command: `m` MOTD, `x` menu, `d` docs, `t` window,
  Tab focus, `v` layout, or `c` close. The footer highlights UNLOCKED until that
  action; Esc or Ctrl+P again locks. Ctrl+G is child-owned, not a Prelude binding. Ctrl+\] / Ctrl+\[ cycle
  floating/left/right/top/bottom without a leader; `Ctrl+\` is the legacy-terminal
  fallback for previous layout. Normal `motd` prints once on entry before the
  first prompt, as main-shell output, not a pane; a startup failure is nonfatal.
  Menu selections close the picker and run in the main Bash, preserving parked
  readline input; if Bash is busy or at PS2, one selection waits for the next
  primary prompt. Catalogue-aware completion loads automatically without ble.sh.
  `x<Tab>` opens a host-rendered chooser below the prompt: Tab/Shift+Tab cycle,
  Enter inserts without running, Esc dismisses; single matches insert directly.
  A later Enter runs the completed command. File/argument completion stays in
  Readline. Key hints use the command menu's padded bold keycaps, muted labels,
  and right-aligned status; only keycaps and the active mode badge paint a
  background. Starship keeps its original bracketed keymap on the right with
  `Alt + [m] motd · [x] menu · [d] docs`. The separate footer shows LOCKED / UNLOCKED
  in all prompt modes; completion keys still use that one protected bottom row. Completion is rendering-only: it preserves
  Bash's PTY size and the full visible Starship prompt, shifting the displayed
  viewport only when space below the prompt is insufficient. For the real Starship prompt, run
  `nix run path:.#ghostty-spike -- --starship`; the fixed prompt stays the default.
  See [`prototypes/ghostty/README.md`](../prototypes/ghostty/README.md).
- **`x sync-docs`** / **`x record-docs`** — documentation workflows.
- **`x demos`** and the other `demos:*` keys (`demos:themes`, `demos:titles`,
  `demos:defaults`) dispatch to the canonical `nix run .#example-*` commands.
