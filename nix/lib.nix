# The flake's `lib` output: curried builders for non-flake-parts users.
#
#   prelude.lib.mkMotd
#     { inherit (pkgs) lib writeShellApplication gum ncurses; }
#     { project = "acme-web"; groups.develop.tasks.dev.run = "pnpm dev"; }
#
# mkMenu takes { lib writeShellApplication writeText buildGoModule } instead.
{
  mkMotd = import ../src/prelude/motd.nix;
  mkMenu = import ../src/prelude/menu.nix;
  themes = import ../src/prelude/themes.nix;
}
