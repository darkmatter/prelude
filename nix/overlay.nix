# Overlay for consumers using `pkgs`:
#
#   nixpkgs.overlays = [ prelude.overlays.default ];
#   # pkgs.prelude.mkMotd / mkMenu — builders taking a config
final: _prev:
let
  deps = {
    inherit (final)
      lib
      writeShellApplication
      writeText
      buildGoModule
      ;
  };
in
{
  prelude = {
    mkTask = import ../src/prelude/task.nix { inherit (final) lib; };
    mkMotd = import ../src/prelude/motd.nix deps;
    mkMenu = import ../src/prelude/menu.nix deps;
    mkDocs = import ../src/prelude/docs.nix deps;
  };
}
