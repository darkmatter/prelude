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

## Install

```sh
bun add @drkmttr/prelude
```

The package is TypeScript source for Bun, plus libprelude, the Go side, built
for macOS and Linux on arm64 and x64. All four libraries ship in the one
package (about 18 MB to download), and it loads this machine's. Linux needs
glibc 2.34 or newer, so it covers Ubuntu 22.04, Debian 12, and RHEL 9 onward,
but not Alpine.

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
  run with bash, with any argument text appended as typed. A command has at
  most one of them, and one with neither only holds subcommands.
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
(`commands: { "db migrate": migrate }`), so modules export commands without
claiming public names. As in the devshell, a space or `/` in a key makes a
subcommand: `db migrate` and `db/migrate` are both `migrate` under `db`, run
as `./main.ts db migrate`, and keys nest deeper (`db seed users`). The menu
shows `db` as one row that opens its subcommands. Mount a command at `db` too
to describe it or give it a group: with `run` or `exec` it also runs itself,
and with neither (`Command.make({ group: "data" })`) it only opens them. `:`
is an ordinary name character.

Keys are never parsed for a group. A command lists under its `group`, or at
the top of the menu, without a heading, when it has none. Only a top-level
command takes a group; a subcommand lists under its parent.

## Surfaces

- `Prelude.make(options)` is one app: `commands` plus optional `motd` and
  `docs`, sharing `project`, `theme`, `palette`, and `colorProfile`. It adds
  the built-in `motd` and `docs` commands. `app.main()` is the command line;
  `app.menu`, `app.motd`, and `app.docs` are the surfaces (the last two only
  when configured).
- `Menu.make(options)` returns a menu with `select()` (the choice, without
  running it), `list()`, `run(selection)`, `launch()` (picker, then run), and
  `dispatch(argv)` (`x` semantics). `launch` and `dispatch` set
  `process.exitCode`. The `just` and `scripts` options import Justfile
  recipes and package.json scripts when the picker opens, as
  `prelude.menu.just` and `prelude.menu.scripts` do. A selection's `source`
  says where its command came from: `run` calls your function only for a
  `declared` one, and runs an import as shell text, in its `dir` with its
  `pathPrefix` ahead of `PATH`.
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

The package loads the path given to `Library.use()`, else `PRELUDE_LIB`, else
the library it ships for this machine under `lib/<platform>-<arch>/`. This
repository's devshell sets `PRELUDE_LIB`; `nix build github:darkmatter/prelude#libprelude` builds the library elsewhere.

A single-file executable embeds the library, so it needs neither Bun nor
`PRELUDE_LIB` where it runs. Import the target platform's library as a file and
pass it to `Library.use()` before the app starts, as
[`standalone.ts`](../examples/typescript/standalone.ts) does:

```ts
import { Library } from "@drkmttr/prelude";
import library from "@drkmttr/prelude/lib/darwin-arm64/libprelude.dylib" with { type: "file" };

Library.use(library);
```

Build once per target platform, importing that platform's library.

## Development

```sh
x ts:test       # bun test (FFI tests need PRELUDE_LIB)
x ts:typecheck  # tsc over src, tests, scripts, and the example
x ts:sync       # regenerate src/internal/generated.ts and the conformance fixtures
```

Change dependencies with the devshell's Bun (`bun install` inside
`nix develop`). The lockfile it writes is one both it and CI's newer Bun
accept; a lockfile written by a newer Bun can't be read by the devshell's.

Public modules sit in `src/`; helpers they share sit in `src/internal/`, which
the namespaces do not expose. The config builders port Nix rules:
`src/internal/catalogue.ts` from `command-catalogue.nix` and `menu.nix`, and
`src/Motd.ts` from `motd.nix`. `test/conformance.test.ts` compares their
output with the JSON Nix generates for the same fixtures. When those Nix files
change, update the port and run `x ts:sync`; `x check` fails while the
generated inputs are stale.

[`scripts/npm-package.ts`](scripts/npm-package.ts) builds the libraries the
package ships:

```sh
bun scripts/npm-package.ts library   # this machine's library → lib/<platform>-<arch>/
bun scripts/npm-package.ts check     # fail unless all four platforms' libraries are in lib/
bun scripts/npm-package.ts smoke     # install the packed package and load the menu
```

CI builds the linux-x64 library, tests against it, and runs `smoke` on every
push, beside `nix flake check`.

## Release

Releases are cut by CI from pushed version tags, as in
[adhere](https://github.com/darkmatter/adhere). From a clean, up-to-date
`main`, push the release tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The tag push starts `.github/workflows/release.yml`, which checks out `main`,
derives `0.1.0` from `v0.1.0`, and runs `bun run release -- --ci 0.1.0` in
`ts/`. Release-it bumps `ts/package.json`, commits `chore: release v0.1.0`
back to `main`, skips npm publish, and creates the GitHub Release for the
existing tag.

Publishing stays in `.github/workflows/publish.yml`: after release-it
finishes, `release.yml` calls the reusable publish workflow directly, avoiding
a chained GitHub Release event created by `GITHUB_TOKEN`. libprelude needs
cgo, so the publish workflow first runs `.github/workflows/libraries.yml`,
which builds each platform's library on its own runner (both macOS libraries
on the arm64 Mac), and collects them into `lib/`.
Then it verifies that `ts/package.json` matches the release tag, checks that
all four libraries are there, tests against the linux-x64 one, loads the
packed package the way an install does, and publishes `@drkmttr/prelude` with
`npm publish --access public --provenance`. No npm token is used, so the
package has to be published once by hand before it can be given trusted
publishers on npmjs.com. npm checks the workflow that started the run, so it
needs two: `release.yml`, which calls the publish workflow for tag releases,
and `publish.yml`, for manual runs. The publish workflow can be run manually
for an already-created release tag if a publish needs to be retried; it skips
a version that is already on npm.

### From a checkout

The first publish happens by hand, since npm has to know a package before it
takes trusted publishers. Run the libraries workflow, download its four
libraries into `lib/`, and publish:

```sh
gh workflow run libraries.yml --ref main
run=$(gh run list --workflow libraries.yml --limit 1 --json databaseId --jq '.[0].databaseId')
gh run watch "$run" && gh run download "$run" --dir ts/lib
cd ts && bun scripts/npm-package.ts check && npm publish
```

That publishes `ts/package.json`'s version, `0.0.0` until the first release
tag bumps it, which is enough to create the package; release tags publish from
then on.
