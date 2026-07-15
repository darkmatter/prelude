# Copyable TypeScript-project usage: expose every package.json script as a
# Prelude menu task without duplicating the script catalogue in Nix.
{ lib, ... }:
let
  package = builtins.fromJSON (builtins.readFile ./package.json);
  scripts = package.scripts or { };
in
{
  prelude = {
    project = package.name or "typescript-app";
    menu.enable = true;

    groups.package-scripts = {
      title = "package.json scripts";
      tasks = lib.mapAttrs (name: command: {
        run = "npm run ${name}";
        description = command;
      }) scripts;
    };
  };
}
