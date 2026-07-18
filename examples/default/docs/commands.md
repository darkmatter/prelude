# Commands

Prelude supplies these whenever the components are enabled:

- **`menu`** (`m`) — fuzzy-filter picker over the catalogue; `menu list`
  prints the table, `menu help` renders a man-style manual.
- **`motd`** (`?`) — reprints the welcome banner.
- **`help`** (`h`) — command help (`menu help`).
- **`docs`** (`d`) — this viewer.

Project commands declared in `examples/default/menu.nix`:

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
