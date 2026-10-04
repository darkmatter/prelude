# Commands

Prelude supplies these whenever the components are enabled:

- **`x`** (`m`) — public catalogue entrypoint. `x` alone opens the interactive
  picker; `x <key>` runs a catalogue command (e.g. `x go:test`, `x docs`).
  `x --list` prints the table non-interactively.
- **`motd`** (`?`) — reprints the welcome banner.
- **`docs`** (`d`) — this viewer (`x docs`).
- **`portal`** (`p`) — app launcher with live health lights.

Project commands declared in `nix/internal/prelude.nix`:

- **`x go:test`**, **`x go:vet`** — public catalogue commands listed under the
  `go` group; they dispatch to canonical `go test -C src ./...` / `go vet -C src ./...` without generating duplicate executables.
- **`x check`** — `nix flake check`: builds every package and render check.
- **`x fmt`** — `treefmt` (alejandra for Nix, gofmt/goimports for Go) over the repository.
- **`x build <target>`** — `nix build` with flake-output suggestions.
- **`x prelude:previews`** — build the render checks and display their output.
- **`x sync-docs`** / **`x record-docs`** — documentation workflows.
- **`x demos`** tours every feature demo; its subcommands (`x demos themes`,
  `x demos titles`, `x demos defaults`) dispatch to the canonical
  `nix run .#example-*` commands. In the menu, `demos` is one row: Enter runs
  the tour and → opens the subcommands.
