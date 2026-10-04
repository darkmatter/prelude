# Prelude in a flake without flake-parts. `prelude.lib.evalModule` evaluates
# the same configuration `flakeModules.default` imports and answers the same
# packages, so a plain `outputs` function, blueprint, or any other framework
# adds `prelude-shell` to its devshell the same way.
{
  description = "Prelude without flake-parts";

  inputs = {
    prelude.url = "github:darkmatter/prelude";
    nixpkgs.follows = "prelude/nixpkgs";
  };

  outputs = {
    nixpkgs,
    prelude,
    ...
  }: let
    systems = [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ];
    forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
  in {
    devShells = forAllSystems (pkgs: let
      shell = prelude.lib.evalModule pkgs ./prelude.nix;
    in {
      default = pkgs.mkShell {
        packages = [shell.packages.prelude-shell];
      };
    });
  };
}
