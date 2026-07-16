# Higher-order flake-parts perSystem adapter. The wrapped function returns the
# canonical output families it normally would; Prelude derives commands from
# those outputs and augments each declared dev shell.
{ lib }:
let
  commandsFromOutputs = import ./output-commands.nix { inherit lib; };
in
userPerSystem:
assert lib.assertMsg (builtins.isFunction userPerSystem)
  "prelude.lib.wrapPerSystem: expected a perSystem function";
args@{
  config,
  pkgs,
  system,
  ...
}:
let
  outputs = userPerSystem args;
  generatedCommands = commandsFromOutputs {
    inherit pkgs system outputs;
  };
  explicitPrelude = outputs.prelude or { };
  explicitCommands = explicitPrelude.commands or { };

  availablePackages = config.packages or { };
  hasMotd = availablePackages ? motd;
  hasMenu = availablePackages ? menu;
  hasDocs = availablePackages ? docs;
  hasPrompt = availablePackages ? prompt;

  # packages.motd already carries packages.menu when the menu is enabled.
  preludeShellPackages =
    lib.optional hasMotd availablePackages.motd
    ++ lib.optional (!hasMotd && hasMenu) availablePackages.menu
    ++ lib.optional hasDocs availablePackages.docs;

  wrapShell =
    shell:
    shell.overrideAttrs (old: {
      nativeBuildInputs = lib.unique ((old.nativeBuildInputs or [ ]) ++ preludeShellPackages);
      shellHook = lib.concatStringsSep "\n" (
        lib.optional ((old.shellHook or "") != "") (old.shellHook or "")
        ++ lib.optional hasPrompt "export STARSHIP_CONFIG=${availablePackages.prompt}"
        ++ lib.optional hasMotd "motd >&2"
      );
    });
in
outputs
// {
  prelude = explicitPrelude // {
    # Recursive merging lets a small explicit override (for example only a
    # description or key) retain the generated invocation and runtime package.
    commands = lib.recursiveUpdate generatedCommands explicitCommands;
  };
}
// lib.optionalAttrs (outputs ? devShells) {
  devShells = lib.mapAttrs (_name: wrapShell) outputs.devShells;
}
