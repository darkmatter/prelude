# Command conventions

Prelude provides one command catalogue without replacing the tools that already
own project workflows.

## One public entrypoint

Every menu entry is runnable through `x` using its complete command key:

```sh
x                 # open the interactive menu
x test             # run an ungrouped command
x go:test          # run a grouped command
x test:unit:watch  # source-owned colons remain valid
x --list           # list available commands
x --help           # show command help
```

The interactive picker and `x` are two views of the same catalogue. Menu-only
commands are not allowed. Selecting an entry interactively and invoking its key
through `x` reaches the same dispatcher and canonical command.

Prelude generates only the `x` dispatcher, not one executable per catalogue
entry. This keeps `PATH` small and avoids synthetic aliases such as `go-test`.

## The key is the public identity

A command key is globally unique and is also its public `x` name. No separate
namespace, discriminator, or label is needed; where a command came from is not
part of its key.

The first colon derives menu presentation without changing the key:

```text
go:test
│  └── displayed command: test
└───── menu group: go

public invocation: x go:test
```

A slash does the same, for keys that read better as a path:

```text
db/migrate
│  └── displayed command: migrate
└───── menu group: db

public invocation: x db/migrate
```

Only the first `:` or `/` is structural. The remainder stays intact,
separators included:

```text
test:unit:watch
│    └──────── displayed command: unit:watch
└───────────── menu group: test

public invocation: x test:unit:watch
```

An ungrouped key such as `build` has no group: the menu lists it at the top,
above every heading, alphabetically with the other ungrouped commands.
Prelude-owned navigation commands appear in `prelude`.

### Explicit group override

When the key name should stay flat but the command belongs under a named
menu group, set `group` on the command:

```nix
prelude.commands.lint = {
  group = "quality";
  exec = "eslint .";
  description = "lint the project";
};
```

`lint` stays on PATH (no `:` or `/` → `grouped` is false) and is callable as
`x lint`, but the menu places it under `quality` instead of at the top. The
override also applies to grouped keys: setting `group = "ci"` on `go:test`
moves it to `ci` while keeping the `x go:test` dispatch form, and
`group = ""` lists it at the top without a heading.

Because keys are unique within the catalogue, command resolution is exact and
deterministic: there is no discriminator syntax. When an import produces a key
that is already declared, the declaration keeps it (see
[Precedence](#precedence)).

## Keep canonical commands canonical

The tool that owns a workflow also owns its underlying invocation:

| Catalogue key | Canonical invocation |
| ------------- | -------------------- |
| `test` | `bun run test` |
| `check` | `just check` |
| `deploy` | `nix run .#deploy` |
| `go:test` | `go test ./...` |

`x` dispatches to these commands; it does not translate every workflow into
`nix run`. The Nix devshell provides dependencies and environment. Once inside
that shell, Prelude invokes the owning tool directly.

The MOTD advertises project commands bare — each ungrouped command is on PATH,
so the row matches what you type — and always lists bare `x` so the command
palette is discoverable from the banner. Grouped keys (`go:test`) have no PATH
entry, so those rows keep the `x go:test` dispatch form. If another command
shadows a bare name, `x <name>` still runs the catalogue command; the banner
notes this under the list. Command details may show the canonical invocation
(`go test ./...`).

## Import; do not export

Existing project files remain authoritative:

- import scripts from `package.json`;
- import recipes from `Justfile`;
- import apps from flake outputs;
- merge explicit Prelude commands for workflows without another owner.

Imported names become command keys verbatim. A package script named `test:unit`
therefore becomes `x test:unit`; Prelude does not reject, rename, or normalize
it. Its first colon (or slash) also organizes the menu under `test`.

Do not write generated entries back to source files, and do not copy imported
commands into `prelude.commands`. Imports are one-way.

### Precedence

Declared commands (`prelude.commands`, or a TypeScript app's own commands)
come first, then Justfile recipes, then `package.json` scripts. When two
produce the same key, the earlier one keeps it, so declaring a key is how you
override an import. The hidden entry is reported, not dropped silently:
`x --list` prints a note under the table,

```text
test: just recipe hidden by declared command
```

and the declared command's details in the picker say `hides just recipe test`.
A declared shortcut counts as a claimed key too, because `x <word>` tries names
before shortcuts: an imported entry named like a declared shortcut is hidden
the same way. Prelude's own commands bring three shortcuts, `m` (`x`), `d`
(`docs`), and `p` (`portal`), so a Justfile `alias d := deploy` is hidden while
`docs` is enabled. Free a shortcut by clearing it, for example
`prelude.commands.docs.key = null;`.

### Justfile import

Set `prelude.menu.just.enable = true` to import public recipes at menu runtime.
Prelude runs:

```sh
just --dump --dump-format json
```

and merges the parsed recipes with the Nix catalogue. A configured
`prelude.commands.<name>` entry wins a recipe with the same name, as
[Precedence](#precedence) describes. The imported recipe keeps
`just <recipe>` as its canonical invocation.

A just module (`mod database 'database.just'`) imports as one menu row:
selecting `database` in the picker — or running `x database` — opens a
subcommand picker over the module's recipes, so a module declutters the menu
instead of adding one row per recipe. Module recipes are invoked through parent
dispatch (`x database migrate` or `x <module> [submodule…] <recipe> [args…]`),
which executes the recipe's canonical command (`just database::migrate`).
Imported module recipes cannot be selected through double-colon keys
(`x database::migrate`). An explicit `[group(…)]` attribute on a
module recipe keeps that recipe displayed in the named top-level group in the
menu while remaining reachable through parent routing (`x database migrate`).
Extras the parent does not recognize keep just's module-dispatch form
(`just database <recipe> <args…>`).

By default, `just` discovers the Justfile from the current working directory.
Set `prelude.menu.just.justfile` to pin a specific Justfile path. Recipes marked
private (including `_`-prefixed recipes) are omitted. If `just` is unavailable,
the Justfile is missing, or parsing fails, the menu keeps the declared commands
and its other imports, and `x --list` says the recipes are unavailable.

### package.json import

Set `prelude.menu.scripts.enable = true` to import the `scripts` of a
`package.json` at menu runtime. The menu reads the nearest `package.json` at or
above the working directory; set `prelude.menu.scripts.packageJson` to pin one
(a relative path resolves from the project root, the directory holding
`flake.nix`).

Each script runs exactly as written, with no package manager in front:
`x build --watch` runs the `build` script's text with `--watch` appended, as
`npm run build -- --watch` would. It runs from its `package.json` directory,
with `node_modules/.bin` from there up to the filesystem root ahead of `PATH`,
which is how `"test": "vitest"` finds `vitest`. Because no package manager runs
it, `pre`/`post` scripts do not run and `npm_*` variables are not set.

Script names become keys verbatim, and a name without `:` or `/` lands in the
`scripts` group (`prelude.menu.scripts.group`). If the `package.json` is
missing or cannot be parsed, `x --list` says so under the table and the rest of
the menu stays.

## Ownership boundaries

```text
nix develop
  provides tools, packages, and environment

x
  resolves an exact catalogue key and dispatches

bun / just / nix run / native CLI
  executes the canonical workflow
```

Nix owns environment construction. Existing tools own task semantics. `x` owns
discovery and dispatch. This avoids turning Prelude into a second package-script
system or task graph.
