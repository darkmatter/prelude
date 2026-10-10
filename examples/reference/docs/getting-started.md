# Getting started

This development shell demonstrates the complete Prelude integration. Keep
`.envrc` as `use flake`: `cd` + direnv provides the lightweight environment, MOTD,
and theming for an already-initialized Starship prompt. Run interactive
`nix develop` for the full footer and catalogue completion; generated project
init enters the enabled workspace in the foreground in Bash with a TTY, with
Starship following `prelude.prompt.enable`. No extra Prelude Bash rc hook is
required.

## Commands

- Run `x` to search Prelude's built-in navigation and the configured project commands.
- Run `docs` to browse these pages.
- Run `x prelude:workspace --starship` to reopen the workspace manually after exit.
- Run `hello` to execute the example application.
- Run `nix flake check` to build the example check.

Prelude provides `x` and `docs`; this example only defines the project-specific `hello` command.
