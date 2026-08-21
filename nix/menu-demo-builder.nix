# Final menu demo package shared by runnable examples and docs captures.
# Uses the prelude repo's own menu config so demos reflect the real dogfood
# configuration rather than the acme-web example fixture.
{
  pkgs,
  lib,
  currentMenuConfig,
}: let
  mkMenu = import ../src/prelude/menu.nix {
    inherit lib;
    inherit
      (pkgs)
      writeShellApplication
      writeText
      buildGoModule
      symlinkJoin
      ;
  };
in {
  inherit mkMenu;
  package = mkMenu currentMenuConfig;
}
