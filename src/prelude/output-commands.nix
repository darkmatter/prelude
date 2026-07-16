# Adapt canonical per-system flake outputs into Prelude commands without forcing
# their values. Apps run, packages build, and checks build their explicit check
# installable; `default` aliases are omitted.
{ lib }:
let
  mkCommand = import ./task.nix { inherit lib; };

  outputNames =
    outputs: family:
    builtins.filter (name: name != "default") (builtins.attrNames (outputs.${family} or { }));

  outputRef =
    system: family: name:
    ".#${family}.${system}.${builtins.toJSON name}";

  commandsFor =
    {
      outputs,
      pkgs,
      system,
      family,
      prefix,
      action,
      group,
      order,
    }:
    lib.listToAttrs (
      map (
        name:
        lib.nameValuePair "${prefix}:${name}" (mkCommand {
          package = pkgs.nix;
          arguments = [
            action
            (outputRef system family name)
          ];
          description = "${action} the ${name} ${lib.removeSuffix "s" family}";
          inherit group order;
        })
      ) (outputNames outputs family)
    );
in
{
  pkgs,
  system,
  outputs,
}:
lib.mergeAttrsList [
  (commandsFor {
    inherit outputs pkgs system;
    family = "apps";
    prefix = "run";
    action = "run";
    group = "run";
    order = 100;
  })
  (commandsFor {
    inherit outputs pkgs system;
    family = "packages";
    prefix = "build";
    action = "build";
    group = "build";
    order = 200;
  })
  (commandsFor {
    inherit outputs pkgs system;
    family = "checks";
    prefix = "check";
    action = "build";
    group = "check";
    order = 300;
  })
]
