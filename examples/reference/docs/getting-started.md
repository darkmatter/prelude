# Getting started

This development shell demonstrates the complete Prelude integration. It enables
workspace entry in the foreground from generated project init in interactive
Bash with a TTY, with Starship following `prelude.prompt.enable`. With direnv,
keep `.envrc` as `use flake`; the existing Bash rc `eval "$(prelude hook bash)"`
sources the init after direnv.

## Commands

- Run `x` to search Prelude's built-in navigation and the configured project commands.
- Run `docs` to browse these pages.
- Run `x prelude:workspace --starship` to reopen the workspace manually after exit.
- Run `hello` to execute the example application.
- Run `nix flake check` to build the example check.

Prelude provides `x` and `docs`; this example only defines the project-specific `hello` command.
