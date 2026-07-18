# Final menu demo package shared by runnable examples and docs captures.
{ pkgs, lib }:
let
  ex = import ../src/prelude/examples.nix;
  mkMenu = import ../src/prelude/menu.nix {
    inherit lib;
    inherit (pkgs)
      writeShellApplication
      writeText
      buildGoModule
      symlinkJoin
      ;
  };
in
{
  inherit mkMenu;
  package = mkMenu ex.menu;
}
