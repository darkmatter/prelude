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

The menu and `x` are two views of the same catalogue. Menu-only commands are not
allowed. Selecting an entry interactively and invoking its key through `x`
reaches the same dispatcher and canonical command.

Prelude generates only the `x` dispatcher, not one executable per catalogue
entry. This keeps `PATH` small and avoids synthetic aliases such as `go-test`.

## The key is the public identity

A command key is globally unique and is also its public `x` name. No separate
source, discriminator, or label is needed.

The first colon derives menu presentation without changing the key:

```text
go:test
│  └── displayed command: test
└───── menu group: go

public invocation: x go:test
```

Only the first colon is structural. The remainder stays intact:

```text
test:unit:watch
│    └──────── displayed command: unit:watch
└───────────── menu group: test

public invocation: x test:unit:watch
```

An ungrouped key such as `build` appears in the default `develop` group.
Prelude-owned navigation commands appear in `prelude`.

Because keys are globally unique, command resolution is exact and deterministic:
there is no collision priority or discriminator syntax.

## Keep canonical commands canonical

The tool that owns a workflow also owns its underlying invocation:

| Catalogue key | Canonical invocation |
| ------------- | -------------------- |
| `test`        | `bun run test`       |
| `check`       | `just check`         |
| `deploy`      | `nix run .#deploy`   |
| `go:test`     | `go test ./...`      |

`x` dispatches to these commands; it does not translate every workflow into
`nix run`. The Nix devshell provides dependencies and environment. Once inside
that shell, Prelude invokes the owning tool directly.

The MOTD advertises the concise catalogue invocation (`x go:test`) for project
commands, while always listing bare `menu` (no `x` prefix) so the command
palette is discoverable from the banner. Command details may show the canonical
invocation (`go test ./...`).

## Import; do not export

Existing project files remain authoritative:

- import scripts from `package.json`;
- import recipes from `Justfile`;
- import apps from flake outputs;
- merge explicit Prelude commands for workflows without another owner.

Imported names become command keys verbatim. A package script named `test:unit`
therefore becomes `x test:unit`; Prelude does not reject, rename, or normalize
it. Its first colon also naturally organizes the menu under `test`.

Do not write generated entries back to source files, and do not copy imported
commands into `prelude.commands`. Imports are one-way.

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
