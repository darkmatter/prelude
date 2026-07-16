# The flake's `lib` output: canonical-output adapters, command constructor,
# and curried builders for non-flake-parts users.
#
#   prelude.lib.mkMotd
#     { inherit (pkgs) lib writeText buildGoModule; }
#     { project = "acme-web"; commandCatalog.dev.exec = "pnpm dev"; }
#
# mkMenu additionally takes { writeShellApplication }.
{ lib }:
{
  mkCommand = import ../src/prelude/task.nix { inherit lib; };
  commandsFromOutputs = import ../src/prelude/output-commands.nix { inherit lib; };
  wrapPerSystem = import ../src/prelude/wrap-per-system.nix { inherit lib; };
  # Compatibility alias for callers migrating from the grouped task schema.
  mkTask = import ../src/prelude/task.nix { inherit lib; };
  mkMotd = import ../src/prelude/motd.nix;
  mkMenu = import ../src/prelude/menu.nix;
  mkDocs = import ../src/prelude/docs.nix;
  themes = import ../src/prelude/themes.nix;
}
