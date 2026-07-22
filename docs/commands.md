# Commands

Prelude supplies these whenever the components are enabled:

- **`x`** — public catalogue entrypoint. `x` alone opens the menu; `x <key>`
  runs a catalogue command (e.g. `x go:test`, `x docs`).
- **`menu`** (`m`) — interactive picker only; `x --list` prints the table
  non-interactively. Prefer `x` for day-to-day runs.
- **`motd`** (`?`) — reprints the welcome banner.
- **`docs`** (`d`) — this viewer (`x docs`).

Project commands declared in `nix/internal/prelude.nix`:

- **`x go:test`**, **`x go:vet`** — public catalogue commands grouped under
  `go`; they dispatch to canonical `go test -C src ./...` / `go vet -C src
./...` without generating duplicate executables.
- **`x check`** — `nix flake check`: builds every package and render check.
- **`x fmt`** — `nixfmt` over the repository.
- **`x build <target>`** — `nix build` with flake-output suggestions.
- **`x previews`** — build the render checks and display their output.
- **`x sync-docs`** / **`x record-docs`** — documentation workflows.
- **`x demos:all`** and the other `demos:*` keys dispatch to the canonical
  `nix run .#example-*` commands.
