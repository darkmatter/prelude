# The flake's `lib` output: task constructor plus curried builders for
# non-flake-parts users.
#
#   prelude.lib.mkMotd
#     { inherit (pkgs) lib writeText buildGoModule; }
#     { project = "acme-web"; groups.develop.tasks.dev.run = "pnpm dev"; }
#
# mkMenu additionally takes { writeShellApplication }.
{ lib }:
{
  mkTask = import ../src/prelude/task.nix { inherit lib; };
  mkMotd = import ../src/prelude/motd.nix;
  mkMenu = import ../src/prelude/menu.nix;
  mkDocs = import ../src/prelude/docs.nix;
  themes = import ../src/prelude/themes.nix;
}
