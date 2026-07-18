# ==============================================================================
# flake.nix
#
# prelude — a flake-parts module suite for devshell UI:
#
#   motd  — static devshell welcome banner (Go + Lip Gloss): header, description,
#           optional env chips, next steps, recipes, and shortcuts
#   menu  — interactive command menu (bubbletea TUI, configured by Nix)
#
# This flake dogfoods its own module: `flakeModules.default` is created with
# flake-parts' importApply, imported below, and configured via root `prelude.nix`
# (the same shape a consumer gets from setup) — `nix develop` greets you with our
# own motd and `menu` drives the project.
#
# Downstream usage (see src/prelude/module.nix for the full option set):
#
#   outputs = { prelude, flake-parts, ... }@inputs:
#     flake-parts.lib.mkFlake { inherit inputs; } {
#       imports = [ prelude.flakeModules.default ];
#       systems = [ "x86_64-linux" "aarch64-darwin" ];
#       prelude = {
#         theme = "phosphor";
#         project = "acme-web";
#         commands.dev = { exec = "pnpm dev"; };
#         motd.enable = true;
#         menu.enable = true;
#       };
#     };
#
# Entry points:
#
#   nix develop .        # devshell greeted by our own motd; `menu` inside
#   nix run .#motd / .#menu / .#previews / .#examples / .#example-*
#   nix flake check      # build + render smoke tests
#
# Layout: flake outputs are one file per output under nix/; root `prelude.nix`
# is the dogfood configuration (imports nix/prelude-*.nix). docs/ holds the
# docs-viewer pages plus guides and generated references.
#   per-system.nix       packages/apps/devshell/checks composition root
#   overlay.nix, lib.nix flake-level outputs
# Component sources live in src/prelude (Nix generators) and internal/menu
# (Go TUI).
# ==============================================================================
{
  description = "Darkmatter devshell UI suite (motd, menu) — flake-parts module";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    treefmt-nix.url = "github:numtide/treefmt-nix";
  };

  outputs =
    inputs:
    inputs.flake-parts.lib.mkFlake { inherit inputs; } (
      { self, flake-parts-lib, ... }:
      let
        # importApply keeps the exported module's error locations pointing at
        # module.nix and lets it close over this flake (localFlake).
        preludeModule = flake-parts-lib.importApply ./src/prelude/module.nix {
          inherit flake-parts-lib;
          localFlake = self;
        };
        # PROTOTYPE: parallel module, intentionally not part of preludeModule.
        motdShellExperimentModule = flake-parts-lib.importApply ./src/experimental/motd-shell/module.nix {
          localFlake = self;
        };
        preludeLib = import ./nix/lib.nix { lib = inputs.nixpkgs.lib; };
      in
      {
        systems = [
          "x86_64-linux"
          "aarch64-linux"
          "x86_64-darwin"
          "aarch64-darwin"
        ];

        # Dogfood like a consumer: module + prelude.nix next to it.
        imports = [
          preludeModule
          motdShellExperimentModule
          ./prelude.nix
          inputs.treefmt-nix.flakeModule
        ];

        flake = {
          # Canonical flake-parts module output plus the previous compatibility name.
          flakeModules.default = preludeModule;
          flakeModules.prelude = preludeModule;
          flakeModules.motd-shell-experiment = motdShellExperimentModule;

          overlays.default = import ./nix/overlay.nix;
          lib = preludeLib;
        };

        perSystem = import ./nix/per-system.nix;
      }
    );
}
