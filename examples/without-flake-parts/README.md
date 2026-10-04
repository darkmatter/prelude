# Prelude without flake-parts

A consumer flake with no flake-parts: `prelude.lib.evalModule pkgs ./prelude.nix`
evaluates the configuration and answers its packages, the same derivations
`flakeModules.default` builds.

```nix
devShells = forAllSystems (pkgs: let
  shell = prelude.lib.evalModule pkgs ./prelude.nix;
in {
  default = pkgs.mkShell {packages = [shell.packages.prelude-shell];};
});
```

In [blueprint](https://github.com/numtide/blueprint), the same lines go in a
devshell file:

```nix
# devshell.nix (or nix/devshells/default.nix with prefix = "nix/")
{pkgs, inputs, ...}: let
  shell = inputs.prelude.lib.evalModule pkgs ./prelude.nix;
in
  pkgs.mkShell {packages = [shell.packages.prelude-shell];}
```

`prelude.nix` is the file the wizard writes. Modules receive `pkgs`; pass
anything else a module takes, such as `self` or `inputs`, through
`_module.args`:

```nix
prelude.lib.evalModule pkgs {
  imports = [./prelude.nix];
  _module.args = {inherit inputs; self = flake;};
}
```

The flake also exposes the packages as `legacyPackages`, so
`nix run github:darkmatter/prelude#docs` run in this repository opens these docs
with this configuration, and `nix run .#prelude-docs` works too.

From the Prelude repository, evaluate this example against the local checkout:

```sh
nix develop ./examples/without-flake-parts --override-input prelude path:. --no-write-lock-file
```
