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
      symlinkJoin
      ;
  };
in
{
  prelude = {
    fromPkg = import ../src/prelude/from-pkg.nix { inherit (final) lib; };
    mkCommand = import ../src/prelude/task.nix { inherit (final) lib; };
    mkTask = import ../src/prelude/task.nix { inherit (final) lib; };
    mkMotd = import ../src/prelude/motd.nix deps;
    mkMenu = import ../src/prelude/menu.nix deps;
    mkDocs = import ../src/prelude/docs.nix deps;
  };
}
