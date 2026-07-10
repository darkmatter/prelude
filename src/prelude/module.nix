# flake-parts module: the prelude devshell UI suite.
#
#   prelude.motd  — devshell welcome banner: load line, banner (status
#                   chips, description), env/git row, next steps, recipes,
#                   footer (static; print from shellHook)
#   prelude.menu  — interactive command menu: fuzzy filter, arg entry, exec
#
# Shared config covers theme/palette and project identity. `groups` belongs to
# the menu; MOTD guidance is authored independently with commands and recipes.
# Options are declared in ./options/{shared,motd,menu}.nix.
#
#   outputs = { prelude, flake-parts, ... }@inputs:
#     flake-parts.lib.mkFlake { inherit inputs; } {
#       imports = [ prelude.flakeModules.prelude ];
#
#       prelude = {
#         theme = "phosphor";
#         project = "acme-web";
#         motd.banner.tagline = "everything you need to build, test & ship";
#
#         groups.develop = {
#           order = 100;
#           tasks.dev = {
#             run = "pnpm dev";
#             description = "start the dev server with hot reload";
#             key = "d";
#             args = [
#               { token = "--port"; description = "Port to bind"; options = [ "3000" "8080" ]; }
#             ];
#           };
#         };
#
#         motd.enable = true;
#         menu.enable = true;
#       };
#
#       perSystem = { pkgs, config, ... }: {
#         devShells.default = pkgs.mkShell {
#           packages = [ config.packages.motd config.packages.menu ];
#           shellHook = ''
#             motd
#           '';
#         };
#       };
#     };
#
# The outer function receives static args via flake-parts' `importApply`
# (see flake.nix); consumers should import the applied module from
# `flakeModules.prelude`, not this file directly.
{ localFlake }:
{ lib, config, ... }:
let
  # Currently unused; kept so the exported module can reference the prelude
  # flake itself (per the flake-parts importApply pattern) without a
  # breaking signature change later.
  _unusedLocalFlake = localFlake;

  cfg = config.prelude;

  mkMotd = import ./motd.nix;
  mkMenu = import ./menu.nix;

  # Shared config threaded into every generator.
  shared = {
    inherit (cfg)
      theme
      palette
      colorProfile
      project
      ;
  };
in
{
  imports = [
    ./options/shared.nix
    ./options/motd.nix
    ./options/menu.nix
  ];

  config = {
    prelude.motd.commands = lib.mkIf cfg.menu.enable {
      browse = {
        command = lib.mkDefault "menu";
        description = lib.mkDefault "browse project commands";
        order = lib.mkDefault 100;
      };
      list = {
        command = lib.mkDefault "menu list";
        description = lib.mkDefault "print project commands";
        order = lib.mkDefault 200;
      };
    };

    perSystem =
      { pkgs, ... }:
      let
        deps = {
          inherit (pkgs)
            lib
            writeShellApplication
            writeText
            gum
            ncurses
            buildGoModule
            ;
        };

        motdPkg = mkMotd deps (
          shared
          // {
            inherit (cfg.motd)
              background
              windowBackground
              clearScreen
              margin
              align
              loadLine
              banner
              description
              env
              commands
              recipes
              git
              footer
              footerHint
              width
              maxWidth
              ;
          }
        );

        menuPkg = mkMenu deps (
          shared
          // {
            inherit (cfg) groups;
            inherit (cfg.menu)
              placeholder
              height
              execute
              width
              maxWidth
              ;
          }
        );

        mkApp = pkg: {
          type = "app";
          program = pkgs.lib.getExe pkg;
        };
      in
      lib.mkMerge [
        (lib.mkIf cfg.motd.enable {
          packages.motd = motdPkg;
          apps.motd = mkApp motdPkg;
        })
        (lib.mkIf cfg.menu.enable {
          packages.menu = menuPkg;
          apps.menu = mkApp menuPkg;
        })
      ];
  };
}
