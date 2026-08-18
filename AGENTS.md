# Prelude

Flake-parts module that greets `nix develop` with a MOTD, command picker (`x`),
docs viewer, and themed prompt. Nix owns declarative config; small Go binaries
consume generated JSON.

Domain language and MOTD pipeline terms live in [`CONTEXT.md`](CONTEXT.md).
Use those names (`Preflight`, `Cache`, `Render`, command catalogue). Do not
invent synonyms.

## Layout

| Path                      | Role                                                                      |
| ------------------------- | ------------------------------------------------------------------------- |
| `flake.nix`               | Thin public flake: inputs, `flakeModules.default`, overlay, lib, template |
| `prelude.nix`             | Dogfood sidecar (same shape a consumer gets from the wizard)              |
| `nix/`                    | Flake output composition, render checks, Python PTY tests                 |
| `nix/internal/`           | This repo's MOTD/menu/docs identity, imported by `prelude.nix`            |
| `src/prelude/`            | flake-parts module, options, shell init, fonts                            |
| `src/cmd/`                | Go mains (`motd`, `menu`, `docs`, `title`, `prompt-status`, VT host)      |
| `src/internal/`           | Go surface implementations (MOTD, menu, docs, wizard)                     |
| `src/pkg/`                | Shared Go (palette, manual viewer, UI primitives)                         |
| `docs/`                   | Viewer pages, guides, generated option/showcase markdown                  |
| `examples/`, `templates/` | Consumer fixtures; evaluated as checks                                    |

Import `flakeModules.default`, never `src/prelude/module.nix` directly.

## Commands

Work inside `nix develop` (or direnv). The catalogue is the public interface:

```sh
x                 # interactive picker
x go:test         # Go unit tests → go test -C src ./...
x go:vet          # go vet -C src ./...
x fmt             # format Nix sources
x check           # nix flake check (packages + render + PTY smoke)
x sync-docs       # regenerate option + showcase markdown
x record-docs     # re-record stale VHS showcases, then sync
```

`x <key>` is the only generated dispatcher. Do not add PATH aliases such as
`go-test`. The first colon is menu presentation; the complete key stays the
public name (`x go:test`).

## Architecture

- **Nix → Config JSON.** Options and catalogue live in Nix. Go does not
  re-default Nix-owned policy except MOTD cache TTLs.
- **MOTD:** Preflight (impure, writes Cache) → Render (pure, Config + Cache).
  Render never shells out and must succeed with a sparse UI when cache is cold.
- **Menu / Docs:** own Config JSON; they do not share the MOTD Cache.
- **Catalogue:** `prelude.commands` is the Nix-side whole. Import Justfile /
  `package.json` / flake apps; do not write generated entries back to source.
  Existing tools own the canonical invocation (`go test`, `nix flake check`).
- **Activation:** `eval "$(prelude-preflight)"` is the only shellHook line.
  Wizard writes a sidecar `prelude.nix` and never overwrites `flake.nix`.

When changing catalogue keys, grouping, or MOTD next-steps, read
[`docs/guides/command-conventions.md`](docs/guides/command-conventions.md).

## Verification

After code changes, start narrow and finish with the flake gate:

```sh
x go:test
x go:vet          # if Go sources changed
x fmt             # if Nix sources changed
x check           # before calling the work done
```

User-visible docs or screenshots: `x sync-docs`, and `x record-docs` when media
is stale. Generated files under `docs/reference/` and `docs/generated/` are
owned by those commands.

Go tests sit next to the package they cover. Python PTY tests live in `nix/`
and run only through flake checks.

## Tracking

This repo uses Beads (`bd`) for durable work.

```sh
bd ready
bd show <id>
bd update <id> --claim
bd close <id> --reason="..."
```

Use `bd` for work that must survive a session. Do not create markdown TODO
files as project state. Run `bd prime` when Beads context is missing.
