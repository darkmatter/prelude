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
status row. The Prelude workspace does this automatically; normal activation is
unchanged.

Workspace and project commands (the workspace entry is built in when enabled;
`nix/internal/prelude.nix` declares this repo's project commands):

- **`x go:test`**, **`x go:vet`** — public catalogue commands listed under the
  `go` group; they dispatch to canonical `go test -C src ./...` / `go vet -C src ./...` without generating duplicate executables.
- **`x check`** — `nix flake check`: builds every package and render check.
- **`x fmt`** — `treefmt` (alejandra for Nix, gofmt/goimports for Go) over the repository.
- **`x build <target>`** — `nix build` with flake-output suggestions.
- **`x prelude:previews`** — build the render checks and display their output.
- **`prelude-workspace`** / **`x prelude:workspace`** — explicitly launch the
  libghostty-vt workspace when `prelude.workspace.enable = true` (default: `false`).
  Both `flakeModules.default` and `lib.evalModule` export `packages.prelude-workspace`
  and bundle the active launcher in `prelude-shell`. Menu and MOTD must be enabled
  and Docs must have at least one page; evaluation asserts these prerequisites.
  The devshell launcher uses the current checkout's menu, with the consumer's own
  Docs, MOTD, theme, and completion config. It never launches during activation.
  The workspace has a single themed hints footer and borderless, floating-by-default
  docs/menu window, with no host-imposed backdrop. Menu panes use `x --embedded` automatically.
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
  viewport only when space below the prompt is insufficient. For the real Starship
  prompt, run `prelude-workspace --starship` in the devshell;
  `prelude.prompt.enable` is not required. The fixed `prelude $ ` prompt stays the
  default. The published package honors `prelude.root` (`lib.evalModule` users set
  it to `self`) with `PRELUDE_ROOT` override support; it uses the consumer's published
  menu. See the [workspace guide](guides/workspace.md) for the enable snippet,
  published launches, and runtime limitations.
- **`x sync-docs`** / **`x record-docs`** — documentation workflows.
- **`x demos`** tours every feature demo; its subcommands (`x demos themes`,
  `x demos titles`, `x demos defaults`) dispatch to the canonical
  `nix run .#example-*` commands. In the menu, `demos` is one row: Enter runs
  the tour and → opens the subcommands.
