# The flake's `lib` output: curried builders for non-flake-parts users.
#
#   prelude.lib.mkMotd
#     { inherit (pkgs) lib writeText buildGoModule; }
#     { project = "acme-web"; groups.develop.tasks.dev.run = "pnpm dev"; }
#
# mkMenu additionally takes { writeShellApplication }.
{
  mkMotd = import ../src/prelude/motd.nix;
  mkMenu = import ../src/prelude/menu.nix;
  themes = import ../src/prelude/themes.nix;
}
