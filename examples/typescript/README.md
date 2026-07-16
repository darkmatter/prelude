# TypeScript package-script menu

A copyable Prelude configuration that reads `package.json` and exposes each
script as an `npm run <name>` menu command.

```nix
{ lib, ... }:
let
  package = builtins.fromJSON (builtins.readFile ./package.json);
in
{
  prelude = {
    project = package.name or "typescript-app";
    menu.enable = true;
    commands = lib.mapAttrs (
      name: description: {
        inherit description;
        exec = "npm run ${name}";
        group = "package.json scripts";
      }
    ) (package.scripts or { });
  };
}
```

From the Prelude repository, inspect the generated command list with:

```sh
nix run path:.#example-typescript-menu -- list
```
