# prelude.prompt.* options — themed starship config for the devshell.
#
# `packages.prompt` is a starship.toml themed from the palette. Starship
# re-resolves $STARSHIP_CONFIG on every prompt render and direnv propagates
# env vars, so the devshell only needs `packages.prelude` plus:
#
#   shellHook = ''export STARSHIP_CONFIG=${config.packages.prompt}'';
#
# The aggregate package supplies Starship, ble.sh, and bash-completion when
# this is enabled. Its setup hook sources one idempotent `prelude-init` file in
# the interactive shell that `nix develop` already started. That file renders
# the MOTD, installs catalogue completion, initializes Starship's native ble.sh
# hooks, and moves the generated right prompt into ble.sh's status line.
# Non-interactive direnv evaluation remains inert.
{ lib, ... }:
let
  defaults = import ../defaults.nix;
in
{
  options.prelude.prompt = {
    enable = lib.mkEnableOption "themed starship prompt config (`packages.prompt` = starship.toml)";

    settings = lib.mkOption {
      type = (lib.types.attrsOf lib.types.anything) // {
        description = "TOML value";
      };
      default = defaults.prompt.settings;
      description = ''
        Starship settings merged (recursively) over the themed defaults.
        See <https://starship.rs/config/>.
      '';
      example = {
        add_newline = true;
        format = "$directory$git_branch$character";
      };
    };

    configFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = defaults.prompt.configFile;
      description = ''
        Use this starship.toml verbatim instead of the generated themed config.
        Prelude leaves its right prompt and ble.sh status line fully user-owned.
      '';
    };
  };
}
