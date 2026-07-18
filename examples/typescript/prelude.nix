# Copyable TypeScript-project usage: expose every package.json script as a
# Prelude menu command without duplicating the script catalogue in Nix.
{ lib, ... }:
let
  package = builtins.fromJSON (builtins.readFile ./package.json);
  scripts = package.scripts or { };
in
{
  prelude = {
    project = package.name or "typescript-app";
    menu.enable = true;

    commands = lib.mapAttrs
      (name: description: {
        inherit description;
        exec = "npm run ${name}";
      })
      scripts;
  };
}
