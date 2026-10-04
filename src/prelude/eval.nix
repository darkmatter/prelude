# `prelude.lib.evalModule pkgs module`: Prelude without flake-parts, for
# blueprint, a plain `outputs` function, or any other flake framework. It
# evaluates the same modules as flakeModules.default and builds the same
# packages:
#
#   let prelude = inputs.prelude.lib.evalModule pkgs ./prelude.nix;
#   in pkgs.mkShell { packages = [ prelude.packages.prelude-shell ]; }
#
# `module` is any module: a path such as the wizard's `prelude.nix`, an
# attribute set, or a function. Modules receive `pkgs`, so a package-backed
# command goes straight into `prelude.commands`; there is no `perSystem`.
# Other arguments a module takes (`self`, `inputs`) come through
# `_module.args`:
#
#   inputs.prelude.lib.evalModule pkgs {
#     imports = [ ./prelude.nix ];
#     _module.args = { inherit inputs; self = flake; };
#   }
#
# Answers `config` and `options` (the evaluated module system) and
# `packages`: `prelude-shell` for the devshell, `prelude` for an app, and the
# per-component packages flakeModules.default exposes.
pkgs: module: let
  inherit (pkgs) lib;
  evaluated = lib.evalModules {
    modules = import ./modules.nix ++ [module];
    specialArgs = {inherit pkgs;};
  };
  cfg = evaluated.config.prelude;
in {
  inherit (evaluated) config options;
  packages = import ./packages.nix {
    inherit lib pkgs cfg;
    inherit (cfg) commands;
  };
}
