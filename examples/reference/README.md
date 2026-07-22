# Prelude reference example

A complete, copyable consumer flake demonstrating Prelude's primary components and package-backed commands.

## Usage

From the Prelude repository, evaluate this example against the local checkout:

```sh
nix flake check ./examples/reference --override-input prelude path:. --no-write-lock-file
nix develop ./examples/reference --override-input prelude path:. --no-write-lock-file
```

Inside the development shell:

```sh
menu       # interactive command picker
docs       # Markdown reference viewer
hello      # package-backed example command
```

The shell prints the MOTD automatically. If Starship is initialized by your
shell, Prelude also applies the generated project prompt while the shell is
active.

Run individual flake outputs without entering the shell:

```sh
nix run ./examples/reference#motd --override-input prelude path:. --no-write-lock-file
nix run ./examples/reference#menu --override-input prelude path:. --no-write-lock-file
nix run ./examples/reference#docs --override-input prelude path:. --no-write-lock-file
nix run ./examples/reference#hello --override-input prelude path:. --no-write-lock-file
```

To use this as the starting point for another repository, copy the directory.
Its declared `github:darkmatter/prelude` input works without the local
`--override-input` argument.

## What it demonstrates

- Importing `prelude.flakeModules.default` with flake-parts.
- Enabling the MOTD, command menu, Markdown docs viewer, and Starship prompt.
- Configuring status, environment probes, project commands, and recipes.
- Keeping canonical `packages`, `apps`, and `checks` as ordinary let bindings.
- Adapting one package with `prelude.lib.fromPkg` without duplicating its executable or runtime closure.
- Explicitly composing Prelude's generated packages into a development shell.

Change `prelude.theme` in `flake.nix` to preview another bundled palette.

## Development

Validate formatting and all outputs with:

```sh
nix fmt -- ./examples/reference/flake.nix
nix flake check ./examples/reference --override-input prelude path:. --no-write-lock-file
```

## Contributing

Keep this example runnable and copyable. Prefer representative configuration
over exhaustive option coverage; the generated options reference remains the
authoritative catalogue.

## License

Licensed under the MIT License — see the parent Prelude repository's `LICENSE` file.
