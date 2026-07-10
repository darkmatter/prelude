# Overlay for consumers using `pkgs`:
#
#   nixpkgs.overlays = [ prelude.overlays.default ];
#   # pkgs.prelude.mkMotd / mkMenu — builders taking a config
final: _prev:
let
  deps = {
    inherit (final) lib writeShellApplication writeText gum ncurses buildGoModule;
  };
in
{
  prelude = {
    mkMotd = import ../src/prelude/motd.nix deps;
    mkMenu = import ../src/prelude/menu.nix deps;
  };
}
