# Final MOTD demo packages shared by runnable examples and docs captures.
{ pkgs, lib }:
let
  ex = import ../src/prelude/examples.nix;
  plib = import ../src/prelude/lib.nix { inherit lib; };
  mkMotd = import ../src/prelude/motd.nix {
    inherit lib;
    inherit (pkgs)
      writeShellApplication
      writeText
      buildGoModule
      ;
  };
in
{
  examplePackages =
    lib.mapAttrs' (name: config: lib.nameValuePair "example-${name}" (mkMotd config)) ex.motdDemos
    // {
      example-motd = mkMotd ex.motd;
      example-themes = pkgs.writeShellApplication {
        name = "motd-themes";
        text = lib.concatMapStringsSep "\n" (
          theme: lib.getExe (mkMotd (ex.themeMotd theme))
        ) plib.themeNames;
      };
    };
}
