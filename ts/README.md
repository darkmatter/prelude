# @drkmttr/prelude

Prelude's command menu, MOTD, and docs viewer for Bun apps. Commands are
TypeScript functions (or shell text) declared next to the code they wrap. The
Go surfaces run in-process through `libprelude`, loaded with `bun:ffi`.

```ts
// src/server.ts
import { Command } from "@drkmttr/prelude";

export const serve = (port: number) => {
  /* … */
};

export const preludeCommand = Command.make({
  description: "run the dev server",
  args: [{ token: "--port", description: "port to run on", type: "number", default: 3000 }],
  run: (args) => serve(args.port), // args.port: number
});
```

```ts
#!/usr/bin/env bun
// main.ts
import { Prelude } from "@drkmttr/prelude";
import { preludeCommand as dev } from "./src/server";

await Prelude.make({ project: "acme", commands: { dev } }).main();
```

`./main.ts` opens the picker, `./main.ts dev --port 8080` runs the command
directly, and `./main.ts --list` prints every command.
[`examples/typescript/`](../examples/typescript/) is a complete app with a MOTD,
docs, and a single-file build.

## Modules

Each module is a namespace: `import { Command, Menu } from "@drkmttr/prelude"`.
Surfaces are built with `make`, and a module's main type shares its name
(`Command.Command`, `Menu.Menu`).

| Module | Use |
| --------- | ---------------------------------------------------------------- |
| `Prelude` | `make(options)`: one app with commands, and optionally MOTD and docs |
| `Command` | `make(definition)`: a command; `is(value)`; `Any` for records |
| `Menu` | `make(options)`: the picker and command line over commands |
| `Motd` | `make(options)`: the welcome banner |
| `Docs` | `make(options)`: the docs viewer over Markdown pages |
| `Args` | `parse`, `split`, and the argument types |
| `Palette` | `themes`, `resolve`, and the look options shared by every surface |
| `Library` | `use(path)`: load libprelude from a specific path |

## Commands

`Command.make` takes the fields of `prelude.commands.<key>` in Nix, plus
`run`:

- **Behavior:** `run` is a function called in this process with the parsed
  arguments; a returned number becomes the exit status. `exec` is shell text
  run with bash, with any argument text appended as typed. A command has
  exactly one of them.
- **Arguments:** `--port` is an option, `--open` with `boolean: true` a flag,
  and `<target>` a positional. Values arrive under camelCase names
  (`--dry-run` → `args.dryRun`), typed from the declaration: `type: "number"`
  parses numbers, `options` restricts values (and offers them as chips in the
  picker), and `required` or `default` makes a value non-optional. The picker's
  argument entry and the command line go through the same parser.
- **Presentation:** `description`, `usage`, `details`, `examples`, a
  single-character `shortcut` (Nix: `key`), and `motd`, a position on the
  MOTD's Getting Started list.

A command has no name of its own. Its key comes from where it is mounted
(`commands: { "db:migrate": migrate }`), so modules export commands without
claiming public names. As in the devshell, the first colon picks the menu
group (`db:migrate` → group `db`, label `migrate`), `group` overrides it, and
ungrouped keys fall under `develop`.

## Surfaces

- `Prelude.make(options)` is one app: `commands` plus optional `motd` and
  `docs`, sharing `project`, `theme`, `palette`, and `colorProfile`. It adds
  the built-in `motd` and `docs` commands. `app.main()` is the command line;
  `app.menu`, `app.motd`, and `app.docs` are the surfaces (the last two only
  when configured).
- `Menu.make(options)` returns a menu with `select()` (the choice, without
  running it), `list()`, `run(selection)`, `launch()` (picker, then run), and
  `dispatch(argv)` (`x` semantics). `launch` and `dispatch` set
  `process.exitCode`.
- `Motd.make(options)` returns a banner with `render(size?)` and `print()`.
  Options mirror `prelude.motd.*`. A status `check` or env `probe` is shell
  text (run by Go, as Preflight does) or a function (run here).
- `Docs.make(options)` returns a viewer with `render({ page, width })` (one
  page as `docs <page>` prints it), `open()` (full screen), and `pages()`.
  Pages are Markdown strings. Bundle files as text:
  `import guide from "./guide.md" with { type: "text" }`.

Each made surface exposes the `config` it hands to Go. The picker and the docs
viewer take over the terminal and block until the person leaves them.

## libprelude

The package loads the library named by `PRELUDE_LIB`, or the path given to
`Library.use()`. This repository's devshell sets `PRELUDE_LIB`; elsewhere,
`nix build github:darkmatter/prelude#libprelude` builds it.

A single-file executable embeds the library, so it needs neither Bun nor
`PRELUDE_LIB` where it runs. Import the library as a file and pass it to
`Library.use()` before the app starts, as
[`standalone.ts`](../examples/typescript/standalone.ts) does:

```sh
cp "$PRELUDE_LIB" examples/typescript/libprelude.so
bun build --compile examples/typescript/standalone.ts --outfile acme
```

The library is built with cgo, once per platform.

## Development

```sh
x ts:test       # bun test (FFI tests need PRELUDE_LIB)
x ts:typecheck  # tsc over src, tests, and the example
x ts:sync       # regenerate src/internal/generated.ts and the conformance fixtures
```

Public modules sit in `src/`; helpers they share sit in `src/internal/`, which
the namespaces do not expose. The config builders port Nix rules:
`src/internal/catalogue.ts` from `command-catalogue.nix` and `menu.nix`, and
`src/Motd.ts` from `motd.nix`. `test/conformance.test.ts` compares their
output with the JSON Nix generates for the same fixtures. When those Nix files
change, update the port and run `x ts:sync`; `x check` fails while the
generated inputs are stale.
