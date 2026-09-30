# TypeScript example

`acme` is a small Bun app built with Prelude's TypeScript API
([`ts/`](../../ts/README.md)). It has a command menu, a welcome banner (MOTD),
and a docs page. Each command is declared next to the code it runs.

| File | Shows |
| --------------- | ------------------------------------------------------ |
| `server.ts` | a module that exports its API and a command wrapping it |
| `db.ts` | a TypeScript command with a flag, and a shell command |
| `main.ts` | the app: commands mounted under names, MOTD, docs |
| `guide.md` | the docs page, bundled into the app as text |
| `standalone.ts` | the entry point for a single-file executable |

## Run it

Inside `nix develop`, which sets `PRELUDE_LIB` to the Go library:

```sh
./examples/typescript/main.ts                      # pick a command
./examples/typescript/main.ts dev --port 8080      # run one directly
./examples/typescript/main.ts db:migrate --dry-run
./examples/typescript/main.ts motd                 # built in: the banner
./examples/typescript/main.ts docs                 # built in: the docs viewer
./examples/typescript/main.ts --list
```

## One executable

```sh
cp "$PRELUDE_LIB" examples/typescript/libprelude.so
bun build --compile examples/typescript/standalone.ts --outfile acme
./acme
```

`acme` carries Bun, the app, the docs page, and the Go library, so it runs
where none of them are installed. `nix run .#example-typescript -- --list`
builds and runs it the same way.

## In your own project

`tsconfig.json` resolves `@drkmttr/prelude` to `../../ts` so the example runs
from a checkout. The package is not on npm yet; see
[`ts/README.md`](../../ts/README.md).
