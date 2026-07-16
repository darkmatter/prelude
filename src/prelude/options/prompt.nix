# prelude.prompt.* options — themed starship config for the devshell.
#
# `packages.prompt` is a starship.toml themed from the palette. Starship
# re-resolves $STARSHIP_CONFIG on every prompt render and direnv propagates
# env vars, so the devshell only needs:
#
#   shellHook = ''export STARSHIP_CONFIG=${config.packages.prompt}'';
#
# The prompt re-themes on entry and reverts when direnv unloads. Requires the
# user's shell to already run starship.
{ lib, ... }:
let
  defaults = import ../defaults.nix;
  t = import ../option-types.nix { inherit lib; };
in
{
  options.prelude.prompt = {
    enable = lib.mkEnableOption "themed starship prompt config (`packages.prompt` = starship.toml)";

    settings = lib.mkOption {
      type = (lib.types.attrsOf lib.types.anything) // { description = "TOML value"; };
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

    shortcuts = lib.mkOption {
      type = lib.types.listOf t.shortcutType;
      default = defaults.prompt.shortcuts;
      description = ''
        Shortcut chips on the prompt's footer bar — a full-width `surface`
        line rendered directly above the input (starship cannot draw below
        the cursor), padded edge-to-edge via the `$fill` module. Defaults to
        `prelude.motd.shortcuts` so the prompt mirrors the MOTD footer; set
        `[ ]` to drop the footer line.
      '';
      example = [
        {
          command = "menu";
          alias = "m";
        }
      ];
    };

    configFile = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = defaults.prompt.configFile;
      description = "Use this starship.toml verbatim instead of the generated themed config.";
    };
  };
}
