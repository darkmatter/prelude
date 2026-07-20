# Your own repo

Two ways in.

## Copy the reference example

`examples/reference/` is a complete consumer flake:

```sh
cp -r examples/reference ../my-shell && cd ../my-shell
nix develop   # its input is github:darkmatter/prelude
```

## Add to an existing flake-parts repo

```nix
{
  inputs = {
    prelude.url = "github:darkmatter/prelude";
    nixpkgs.follows = "prelude/nixpkgs";
    flake-parts.follows = "prelude/flake-parts";
  };

  outputs = { prelude, flake-parts, ... }@inputs:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ prelude.flakeModules.default ];
      systems = [ "x86_64-linux" "aarch64-darwin" ];

      prelude = {
        project = "my-app";
        motd.enable = true;
        menu.enable = true;
        docs.pages = [ { text = ./docs/getting-started.md; } ];
        commands.dev = {
          exec = "pnpm dev";
          description = "start the dev server";
        };
      };

      perSystem = { pkgs, config, ... }: {
        devShells.default = pkgs.mkShell {
          packages = [ config.packages.prelude ];
          shellHook = "motd";
        };
      };
    };
}
```

`packages.prelude` bundles every enabled component and its runtime dependencies,
so one package on PATH gives your team the whole surface. When the prompt is
enabled, that includes Starship and ble.sh plus automatic initialization in an
interactive Bash `nix develop` shell. Package-backed commands —
`prelude.lib.fromPkg pkgs.foo { … }` — carry their runtime closure with them,
so they work without adding the tool to the shell.

Details: `README.md` (Usage, Command schema) and the Configuration page here.
