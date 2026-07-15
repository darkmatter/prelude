# TypeScript package-script menu

A copyable Prelude configuration that reads `package.json` and exposes each
script as an `npm run <name>` menu task.

```nix
{ lib, ... }:
let
  package = builtins.fromJSON (builtins.readFile ./package.json);
in
{
  prelude = {
    project = package.name or "typescript-app";
    menu.enable = true;
    groups.package-scripts = {
      title = "package.json scripts";
      tasks = lib.mapAttrs (
        name: command: {
          run = "npm run ${name}";
          description = command;
        }
      ) (package.scripts or { });
    };
  };
}
```

From the Prelude repository, inspect the generated task list with:

```sh
nix run path:.#example-typescript-menu -- list
```
