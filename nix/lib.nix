# The flake's `lib` output: small command adapters plus curried builders for
# non-flake-parts users.
#
#   prelude.lib.mkMotd
#     { inherit (pkgs) lib writeText buildGoModule; }
#     { project = "acme-web"; commandCatalog.dev.exec = "pnpm dev"; }
#
# mkMenu additionally takes { writeShellApplication, symlinkJoin }.
{ lib }:
let
  internal = import ../src/prelude/lib.nix { inherit lib; };
in
{
  fromPkg = import ../src/prelude/from-pkg.nix { inherit lib; };
  mkCommand = import ../src/prelude/task.nix { inherit lib; };
  # Compatibility alias for callers migrating from the grouped task schema.
  mkTask = import ../src/prelude/task.nix { inherit lib; };
  mkMotd = import ../src/prelude/motd.nix;
  mkMenu = import ../src/prelude/menu.nix;
  mkDocs = import ../src/prelude/docs.nix;
  themes = import ../src/prelude/themes.nix;
  # Markdown → docs page leaves ({ title, text = path }) at H2 boundaries.
  inherit (internal) mdSplit;
}
