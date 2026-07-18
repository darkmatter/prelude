# Prelude default example

The flake-parts configuration used by Prelude itself and presented as the repository's default example.

## Usage

Run the example through the repository root, which imports `examples/default`:

```sh
nix develop
nix run .#motd
nix run .#menu -- list
nix run .#docs
```

`default.nix` is a flake-parts module rather than a standalone flake. It composes the component-specific `motd.nix`, `menu.nix`, `docs.nix`, and `prompt.nix` modules. The Markdown pages and checked-in title are colocated with the configuration that consumes them.

## Contributing

Keep this example representative of Prelude's recommended module usage. Repository-specific project commands are appropriate because the root flake dogfoods this exact configuration; Prelude-owned navigation commands and keybindings should remain in the module defaults.

## License

The parent Prelude repository does not currently declare a license.
