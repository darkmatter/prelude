# Prelude

Devshell UI suite that greets `nix develop` with a MOTD, command picker (`x`),
docs viewer, and themed prompt. Nix owns declarative config; small Go binaries
consume generated JSON.

Domain language and MOTD pipeline terms live in [`CONTEXT.md`](CONTEXT.md).
Use those names (`Preflight`, `Cache`, `Render`, command catalogue). Do not
invent synonyms.

## Layout

| Path | Role |
| ---------------------------- | ------------------------------------------------------------------------------------------------ |
| `flake.nix` | Thin public flake: inputs, `flakeModules.default`, overlay, lib, template |
| `prelude.nix` | Dogfood sidecar (same shape a consumer gets from the wizard) |
| `nix/` | Flake output composition, render checks, Python PTY tests |
| `nix/internal/` | This repo's MOTD/menu/docs identity, imported by `prelude.nix` |
| `src/prelude/` | flake-parts module, `lib.evalModule`, the package builder both share, options, shell init, fonts |
| `src/cmd/` | Go mains (`motd`, `menu`, `docs`, `title`, `prompt-status`, VT host, `libprelude`) |
| `src/cmd/prelude-workspace/` | Opt-in Bash workspace; independent Go/C module using libghostty-vt |
| `src/prelude/workspace.nix` | Workspace package and dedicated devshell builder |
| `src/internal/` | Go surface implementations (MOTD, menu, docs, wizard) |
| `src/pkg/` | Shared Go (palette, manual viewer, UI primitives) |
| `docs/` | Viewer pages, guides, generated option/showcase markdown |
| `examples/`, `templates/` | Consumer fixtures; evaluated as checks |
| `ts/` | TypeScript API (`@drkmttr/prelude`, Bun): commands, menu, MOTD, docs over `libprelude` |

Consumers use `flakeModules.default` or `lib.evalModule`, never
`src/prelude/module.nix` directly. Both evaluate `src/prelude/modules.nix` and
build through `src/prelude/packages.nix`, and a check holds their derivations
equal; put package logic there, not in either entry point.

## Commands

Work inside `nix develop` (or direnv). The catalogue is the public interface:

```sh
x                 # interactive picker
x prelude:workspace --starship  # manual workspace with the themed prompt
x go:test         # Go unit tests → go test -C src ./...
x go:vet          # go vet -C src ./...
x fmt             # format Nix sources
x check           # nix flake check (packages + render + PTY smoke)
x sync-docs       # regenerate option + showcase markdown
x record-docs     # re-record stale VHS showcases, then sync
x ts:test         # TypeScript API tests → bun --cwd ts test
x ts:typecheck    # tsc over ts/, including its type tests
x ts:sync         # regenerate ts/ themes, defaults, and conformance fixtures
```

`x <key>` is the only generated dispatcher. Do not add PATH aliases such as
`go-test`. A space or `/` in a key makes a subcommand (`x db migrate`); `:` is
part of the name (`x go:test`), and groups come only from `group`.

## Architecture

- **Nix → Config JSON.** Options and catalogue live in Nix. Go does not
  re-default Nix-owned policy except MOTD cache TTLs.
- **MOTD:** Preflight (impure, writes Cache) → Render (pure, Config + Cache).
  Render never shells out and must succeed with a sparse UI when cache is cold.
- **Menu / Docs:** own Config JSON; they do not share the MOTD Cache.
- **Catalogue:** `prelude.commands` is the Nix-side whole. Import Justfile /
  `package.json` / flake apps; do not write generated entries back to source.
  Existing tools own the canonical invocation (`go test`, `nix flake check`).
- **Workspace:** consumers opt in with `prelude.workspace.enable`. Generated
  project init enters `prelude-workspace` in the foreground in interactive Bash
  with a TTY. `.envrc` stays `use flake`; the existing Bash rc
  `eval "$(prelude hook bash)"` sources the init after direnv. Stamp each init
  before launch: exit/failure must not immediately reopen it; leaving/reentering
  or a changed init permits reentry. Never launch from noninteractive, envrc,
  lorri, zsh, or non-TTY contexts; `PRELUDE_WORKSPACE_ACTIVE` prevents child
  recursion. Workspace mode skips legacy BLE initialization. Automatic Starship
  follows `prelude.prompt.enable`; manual `x prelude:workspace` /
  `prelude-workspace --starship` remain supported. Both entrypoints build consumer
  Config through `packages.nix`: the shell uses the checkout Menu, the published
  workspace honors `prelude.root`. Native compilation is Config-independent;
  disabled consumers exclude its renderer. The repo opts in for dogfooding.
  Its independent Go module and tooling remain separate from the main module.
  See [`docs/guides/workspace.md`](docs/guides/workspace.md).
- **Activation:** `eval "$(prelude-preflight)"` is the only shellHook line.
  Wizard writes a sidecar `prelude.nix` and never overwrites `flake.nix`.
- **TypeScript API:** `ts/` is a second author of the menu and MOTD Config
  JSON. `ts/src/internal/catalogue.ts` and `ts/src/Motd.ts` port the Nix
  rules, and a conformance fixture holds them to Nix's own output. After
  changing `command-catalogue.nix`, `menu.nix`, `motd.nix`, themes, or
  defaults, update the port and run `x ts:sync`; `x check` fails while it is
  stale. Public modules are namespaces with `make` (`Command.make`); shared
  helpers stay in `ts/src/internal/`. The Go surfaces reach it through
  `libprelude` (`src/cmd/libprelude`, cgo c-shared), whose exports never exit
  or exec: the host runs the selection. A pushed `vX.Y.Z` tag on `main`
  publishes it to npm with every platform's library inside; see
  [`ts/README.md#release`](ts/README.md#release).

When changing catalogue keys, grouping, or MOTD next-steps, read
[`docs/guides/command-conventions.md`](docs/guides/command-conventions.md).

## Verification

After code changes, start narrow and finish with the flake gate:

```sh
x go:test
x go:vet          # if Go sources changed
x ts:test         # if ts/ or a Go library entry point (api.go) changed
x fmt             # if Nix sources changed
x check           # before calling the work done
```

User-visible docs or screenshots: `x sync-docs`, and `x record-docs` when media
is stale. Generated files under `docs/reference/` and `docs/generated/` are
owned by those commands.

Go tests sit next to the package they cover. The workspace module is tested and
vetted separately inside `nix develop .#workspace` with
`go test -C src/cmd/prelude-workspace -race ./...` and
`go vet -C src/cmd/prelude-workspace ./...`; the flake gate also builds its check.
Python PTY tests live in `nix/` and run only through flake checks.

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
