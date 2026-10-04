# Command Sources

**Date:** 2026-10-01\
**Status:** Approved design

## Goal

One command catalogue gathers commands from every place a project already keeps them: Nix declarations, a Justfile, `package.json` scripts, flake apps, and a Bun app built on the TypeScript API. Both hosts show the result: the devshell's `x`, and a Bun app over libprelude. Imports stay one-way and verbatim, as [command conventions](../../guides/command-conventions.md) requires.

## Current state

| Source | Brought in by | Devshell `x` | Bun host |
| --- | --- | --- | --- |
| `prelude.commands` (incl. `prelude.lib.fromPkg` / `mkCommand`) | Nix, at eval time | yes | no |
| Justfile recipes | Go at runtime (`importJust` runs `just --dump`) | yes | yes (`Menu.make({ just })`) |
| `Command.make` in a Bun app | the app's own Config | no | yes |
| `package.json` scripts | nothing | no | no |
| flake apps | nothing | no | no |

- The catalogue rules exist twice: `src/prelude/command-catalogue.nix`, and its port `ts/src/internal/catalogue.ts`, held to Nix's output by the menu half of `ts/test/fixtures/conformance*.json`. The rules are identity from the first colon, default groups, group order, label sort, and validation. The Just import adds a third, partial copy: `mergeJustTasks` re-sorts each group, and `taskGroup` re-derives groups from the first colon.
- Precedence is implicit. `mergeJustTasks` keeps whichever task is already in the Config, so a recipe silently disappears behind a declared command with the same key.
- A task can say only *what* to run: `run`, shell text exec'd with `bash -c` from the caller's directory. It cannot say *where* to run, or with what `PATH`. A `package.json` script needs both to run unmodified.
- `docs/guides/command-conventions.md` promises to import `package.json` scripts and flake apps, but neither is implemented. The same guide says there is "no collision priority", which the Just import already contradicts.

## Decisions

1. **Precedence: declared > host > just > scripts > flake.**
   - "Declared" means `prelude.commands` in the devshell, or the commands passed to `Menu.make` / `Prelude.make` in a Bun host.
   - The winning entry keeps the key. Each losing entry is recorded as hidden and reported, never dropped silently. A declared shortcut claims its key the same way, because `x <word>` tries names before shortcuts.
   - Declaring a key on purpose to override an import stays supported: a hidden entry is reported, not an error.
1. **Go owns the catalogue rules.**
   - Identity, default groups, group order, sorting, validation, and precedence run once, in Go, over every source.
   - Nix and TypeScript send raw command declarations.
   - Nix keeps declarations, defaults, PATH wrappers, runtime packages, and its assertions as early errors.
   - This revises the AGENTS.md rule "Options and catalogue live in Nix". Declarations stay in Nix; the rules that merge them move to Go, because most sources are now read at runtime.
1. **Scripts run verbatim.**
   - A `package.json` script runs exactly as written: no package manager, nothing prepended to its text.
   - Extra arguments are appended, as `npm run <name> -- <args>` appends them.
   - The script runs from its `package.json` directory, with each `node_modules/.bin` from there up to the filesystem root ahead of `PATH`. That is how `"test": "vitest"` finds `vitest`, and it is the one thing npm adds that scripts depend on.
   - Lifecycle hooks (`pretest`, `posttest`) don't run, and `npm_*` variables aren't set.
1. **Flake apps are devshell-only, resolved at eval time.**
   - Nix reads app names and `meta.description` from the flake-parts `apps` option and never forces `program`.
   - The Bun host does not import flake apps. It is built to ship as a single-file executable where Nix may be absent, and a runtime `nix flake show` takes seconds.
   - A Bun app that needs a flake app declares `Command.make({ exec: "nix run .#deploy --" })`.
1. **A task carries where it runs, not only what it runs.**
   - Tasks and Selections gain `source`, `dir`, and `pathPrefix`, and hosts apply them when they run shell text.
   - libprelude exports still never exec. Each host's shell runner stays a few lines, so a shared execution export would cost more than it saves.
1. **New options sit beside `prelude.menu.just`.** The new sources are siblings (`prelude.menu.scripts`, `prelude.menu.flakeApps`, `prelude.menu.hosts`), so `menu.just` keeps its name and no consumer has to migrate. The Bun host takes `Menu.make({ just, scripts })`.
1. **No catalogue cache until measured.**
   - `just --dump` and reading `package.json` are cheap enough to run whenever the menu opens, as the Just import already does.
   - A cache is added only if a source, most likely a host manifest, measurably slows down opening `x`.

## Model

### Sources

| `source` | Enabled by | Group for keys without a colon | `run` | `dir` | `pathPrefix` |
| --- | --- | --- | --- | --- | --- |
| `declared` | `prelude.commands`, `Menu.make({ commands })` | none: listed at the top, without a heading | `exec`, else the key | caller's | — |
| `host` | `prelude.menu.hosts.<name>` | `<name>` | `<command> <key>` | project root | — |
| `just` | `prelude.menu.just`, `Menu.make({ just })` | `just.group` (`just`) | `just [--justfile f] <recipe>` | caller's | — |
| `scripts` | `prelude.menu.scripts`, `Menu.make({ scripts })` | `scripts.group` (`scripts`) | the script text | the `package.json` directory | each `node_modules/.bin` upward |
| `flake` | `prelude.menu.flakeApps` | `flakeApps.group` (`flake`) | `nix run .#<app> --` | caller's | — |

- Keys are imported verbatim.
- The first `:` or `/` in a key sets the group for every source (`test:unit` and `test/unit` → `test`), and an explicit group overrides it.
- `x <key>` reaches every entry. Just module recipes keep their parent routing (`x database migrate`).
- The `source` field records where an entry came from; it is not part of the entry's identity.
- `flake` needs no `dir`, because `nix` searches upward from a subdirectory for `flake.nix`. The project root for `host` is found the same way: the nearest ancestor that holds `flake.nix`.

### Task and Selection

```go
type Task struct {
	// … existing fields …
	Source     string   `json:"source,omitempty"`     // "" = declared
	Dir        string   `json:"dir,omitempty"`        // "" = the caller's directory
	PathPrefix []string `json:"pathPrefix,omitempty"` // prepended to PATH, nearest first
}

type Selection struct {
	Name       string   `json:"name"`
	Line       string   `json:"line"`
	Command    string   `json:"command"`
	Source     string   `json:"source"`
	Dir        string   `json:"dir,omitempty"`
	PathPrefix []string `json:"pathPrefix,omitempty"`
}
```

- `x` applies `Dir` and `PathPrefix` before `syscall.Exec`.
- The TypeScript host passes them to `Bun.spawn` as `cwd` and `env.PATH`.
- A Bun host runs a function command in-process only when the Selection's source is `declared`, so a selection that came from an import is never mistaken for one of the host's own functions.
- Phase 2 bumps `prelude_abi_version` to 2: the Config gains `scripts`, which an older library's strict decoder rejects, and every Selection carries `source`.

### Precedence and hidden entries

When two sources produce the same key, the higher-ranked source wins. Imports merge in precedence order, so a name stays with the task that claimed it first, and that task records what it hid. Several hosts rank among themselves by name. Hidden entries show up in two places:

- `x --list` prints one dim line per hidden entry under the table (`test: just recipe hidden by declared command`). That is the channel the failed-import warning already uses.
- The winning task's expanded view in the picker gains `hides just recipe test`.

### Declarations

After phase 4, the menu Config carries declarations, not pre-grouped `groups`:

- A declaration has the Task fields a source knows: `name`, `source`, `run`, `description`, `key` (shortcut), `usage`, `details`, `examples`, and `args`, plus `group` only when set explicitly, and `motd`.
- Go derives `label`, `group`, `command`, group order, and sorting.
- `groupOrder` moves into the menu Config.
- A new export, `prelude_menu_config`, normalizes declarations, or fails on invalid ones, and returns the grouped Config. `Menu.make` calls it, so invalid commands still throw from `make`, and `menu.config` still shows the grouped catalogue. Runtime imports still happen when the picker opens, because they depend on the working directory at that moment.

MOTD rows (`motdCommands` and Getting Started) keep their current authors. They depend on `motd` order and the key, not on grouping.

### Host manifest

A Bun app built on `Prelude.make` or `Menu.make` answers `--prelude-manifest` by printing `{"manifest": 1, "commands": [...]}`.

- The commands use the declaration shape, without `run`: the devshell always hands execution back to the app.
- Only the commands passed in `commands` appear. Built-ins (`motd`, `docs`) and the app's own imports are left out, because the devshell has its own.

```nix
prelude.menu.hosts.acme = {
  command = "bun tools/main.ts"; # run from the project root
  group = "acme"; # default: the attribute name
};
```

- When the menu opens, `x` runs `<command> --prelude-manifest` and imports each command with `run = "<command> <key>"`.
- So `x dev --port 3000` runs `bun tools/main.ts dev --port 3000`, and the app parses its typed arguments itself.
- The manifest's `args` drive the picker's argument entry.
- If the command fails or prints something invalid, the menu keeps the other sources and adds a dim warning, as a failed Just import does.

### Options

| Option | Default | Meaning |
| --- | --- | --- |
| `prelude.menu.just.*` | — | Unchanged. |
| `prelude.menu.scripts.enable` | `false` | Import `package.json` scripts when the menu opens. |
| `prelude.menu.scripts.packageJson` | `null` | String path. `null` means the nearest `package.json` upward from the working directory. A relative path resolves from the project root, or from the working directory when there is no `flake.nix`. A string, not a Nix path, so it is never copied into the store away from `node_modules`. |
| `prelude.menu.scripts.group` | `"scripts"` | Group for script keys without a colon. |
| `prelude.menu.flakeApps.enable` | `false` | Import perSystem `apps`, except `default`, at eval time. |
| `prelude.menu.flakeApps.group` | `"flake"` | Group for app names without a colon. |
| `prelude.menu.hosts.<name>.command` | — | Shell command that starts the app, run from the project root. |
| `prelude.menu.hosts.<name>.group` | `<name>` | Group for the app's keys without a colon. |

```ts
Menu.make({ commands, just: { enable: true }, scripts: { enable: true } });
```

`Menu.make({ scripts })` reaches the same Go import as the devshell through libprelude, as `just` does today.

## Phases

Each phase ships on its own and leaves `x check` green.

1. **Provenance and run context.**
   - Add `source`, `dir`, and `pathPrefix` to Task and Selection, and to `Menu.Selection` in TypeScript.
   - Generalize `mergeJustTasks` into one merge for every import. Imports merge in precedence order, so the task that claims a name first keeps it and records what it hid.
   - Report hidden entries in `x --list` and in the expanded view.
   - `x` and `Menu.run` apply `dir` and `pathPrefix`.
   - Docs: CONTEXT.md (Source, hidden entries, Selection) and the precedence order in command-conventions.md.
1. **`package.json` scripts.**
   - Add `importScripts` in Go, beside `importJust`, and bump `prelude_abi_version` to 2.
   - `x <TAB>` reads imported keys from a hidden `x --imports` instead of running `just --summary` itself, so completion offers exactly what `x` dispatches, scripts included.
   - Add the `prelude.menu.scripts` options and `Menu.make({ scripts })`.
   - Tests: verbatim run text, appended arguments, `dir`, the `node_modules/.bin` chain, colon grouping, and a script hidden by a recipe.
   - Docs: a `package.json` section in command-conventions.md.
1. **Flake apps.**
   - Add the `prelude.menu.flakeApps` options.
   - Nix emits `flakeApps: [{name, description}]` in the menu Config from perSystem `config.apps`, reading names and `meta.description` only. Go builds the `flake` tasks, so collisions go through the same merge.
   - Add a check that enables the import with an app whose `program` throws, proving the import never forces it.
   - Docs: a flake apps section in command-conventions.md.
1. **Catalogue rules into Go.**
   - Move identity, default groups, group order, sorting, and validation into Go.
   - `menu.nix` and `Menu.make` send declarations.
   - Add `prelude_menu_config` and bump `prelude_abi_version` to 3.
   - In Nix, delete `projectMenuGroups` and `normalizeCommandGroups`. Keep `normalizeCommandEntries` and `selectCommands` for PATH wrappers, MOTD rows, and assertions.
   - In TypeScript, delete `catalogue`, `identity`, and `validate`. `gettingStarted` stays, for the MOTD.
   - Delete the menu half of the conformance fixture. Its inputs become Go table tests; the MOTD half stays.
   - Docs: AGENTS.md (Architecture: Nix → Config JSON, TypeScript API) and CONTEXT.md (Command catalogue, TypeScript → Config).
1. **Hosts as a source.**
   - `Menu.dispatch` answers `--prelude-manifest`.
   - Add the `prelude.menu.hosts` options and a Go runtime import.
   - Add a check that the devshell `x` lists `examples/typescript`'s commands.
1. **Catalogue cache, only if measured.**
   - Time opening `x` and `x --list` with each source enabled.
   - If a source adds noticeable latency, give runtime sources a Preflight → Cache → Render path, keyed by their input files (Justfile, `package.json`, host command) and refreshed after paint, as async MOTD status is.

## Out of scope

- Workspaces: one `package.json` per import (the nearest, or `scripts.packageJson`).
- Running npm lifecycle hooks, or setting `npm_*` variables.
- Flake apps in the Bun host, and flake `packages` as commands (use `prelude.lib.fromPkg`).
- Writing imported commands back to their sources.
- A libprelude export that runs commands.

## Verification

Per phase: `x go:test` and `x go:vet`; `x ts:test` and `x ts:typecheck` when `ts/` or `api.go` change (phases 1, 2, 4, 5); `x fmt` for Nix changes; `x sync-docs` when options change; and `x check` before the phase is done.
