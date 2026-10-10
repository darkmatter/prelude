# flake-parts module: the prelude devshell UI suite.
#
#   prelude.motd    — devshell welcome banner
#   prelude.menu    — interactive command menu
#   prelude.docs    — Markdown project docs viewer
#   prelude.prompt  — themed starship config (packages.prelude-prompt = starship.toml)
#   prelude.workspace — opt-in Bash workspace with the project's Menu and Docs
#
# Shared config covers theme/palette, project identity, and a flat command
# catalogue. MOTD guidance and docs content are authored independently.
# Options are declared under ./options/; ./modules.nix lists the shared modules.
#
# Without flake-parts, `prelude.lib.evalModule` (./eval.nix) evaluates the same
# modules (./modules.nix); both build through ./packages.nix.
#
#   outputs = { prelude, flake-parts, ... }@inputs:
#     flake-parts.lib.mkFlake { inherit inputs; } {
#       imports = [ prelude.flakeModules.default ];
#
#       prelude = {
#         theme = "phosphor";
#         project = "acme-web";
#         motd.header.tagline.text = "everything you need to build, test & ship";
#
#         commands.dev = {
#           description = "start the dev server with hot reload";
#           exec = "pnpm dev";
#           group = "develop";
#           key = "d";
#           order = 100;
#         };
#
#         docs.pages = [
#           { text = ./docs/getting-started.md; }
#         ];
#
#         motd.enable = true;
#         menu.enable = true;
#       };
#
#       perSystem = { pkgs, config, ... }: {
#         devShells.default = pkgs.mkShell {
#           packages = [ config.packages.prelude-shell ];
#         };
#       };
#     };
#
# The module exports one self-contained app, `apps.prelude`, plus a
# `packages.prelude-shell` that bundles every enabled component. Add only that
# one package to the consumer devshell; the setup-hook handles activation.
#
# The outer function receives static args via flake-parts' `importApply`
# (see flake.nix); consumers should import the applied module from
# `flakeModules.default`, not this file directly.
{
  localFlake,
  flake-parts-lib,
}: {
  lib,
  config,
  self,
  ...
}: let
  # Currently unused; kept so the exported module can reference the prelude
  # flake itself (per the flake-parts importApply pattern) without a
  # breaking signature change later.
  _unusedLocalFlake = localFlake;

  cfg = config.prelude;
  optionTypes = import ./option-types.nix {inherit lib;};
in {
  imports = import ./modules.nix;

  # Published packages are bound to the flake's own source.
  config.prelude.root = lib.mkDefault self.outPath;

  options.perSystem = flake-parts-lib.mkPerSystemOption (
    {lib, ...}: {
      options.prelude.commands = lib.mkOption {
        type = lib.types.attrsOf optionTypes.commandType;
        default = {};
        description = "System-specific project commands, including package-backed commands created with prelude.lib.fromPkg.";
      };
    }
  );

  # The packages come from ./packages.nix, which lib.evalModule shares. Only
  # the per-system command entries and the app are flake-parts' own.
  config.perSystem = {
    pkgs,
    config,
    ...
  }: let
    packages = import ./packages.nix {
      inherit lib pkgs cfg;
      commands = lib.recursiveUpdate cfg.commands config.prelude.commands;
    };
  in {
    inherit packages;
    # One self-contained app. It uses the embedded dispatcher rather than the
    # PATH-resolving shell core.
    apps.prelude = {
      type = "app";
      program = lib.getExe packages.prelude;
    };
  };
}
