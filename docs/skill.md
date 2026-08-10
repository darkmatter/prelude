# Prelude agent guide

Prelude is a flake-parts devshell UI: a welcome banner, command catalogue,
Markdown docs viewer, and themed prompt. This command prints the upstream
Markdown directly—no TUI, checkout, or network access after Nix resolves the
flake.

Start a project with:

```sh
nix run github:darkmatter/prelude#setup
```

Read a topic with:

```sh
nix run github:darkmatter/prelude#skill -- <topic>
```

Use `nix run github:darkmatter/prelude#skill -- list` before guessing. Topics:

- `install` — setup output, flake-parts wiring, and reference consumer.
- `options` — complete generated `prelude.*` reference.
- `commands` — `x`, `motd`, and `docs` command contracts.
- `configuration` — common option groups and docs configuration.
- `guide command-conventions` — catalogue ownership and dispatch rules.
- `guide title-rendering` — FIGlet title and setup details.
