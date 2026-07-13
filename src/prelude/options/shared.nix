# Top-level prelude.* options: shared visual/project identity plus task groups
# used exclusively by the interactive menu.
{ lib, ... }:
let
  plib = import ../lib.nix { inherit lib; };
  defaults = import ../defaults.nix;
  t = import ../option-types.nix { inherit lib; };
in
{
  options.prelude = {
    theme = lib.mkOption {
      type = lib.types.enum plib.themeNames;
      default = defaults.theme;
      description = "Color theme for all prelude components.";
    };

    palette = lib.mkOption {
      default = { };
      description = "Per-token overrides applied on top of the theme.";
      type = lib.types.submodule {
        options = {
          fg = t.mkColorOption "fg";
          muted = t.mkColorOption "muted";
          dim = t.mkColorOption "dim";
          border = t.mkColorOption "border";
          accentBorder = t.mkColorOption "accentBorder";
          accent = t.mkColorOption "accent";
          accent2 = t.mkColorOption "accent2";
          error = t.mkColorOption "error";
          selectionFg = t.mkColorOption "selectionFg";
          bg = t.mkColorOption "bg";
          surface = t.mkColorOption "surface";
          secondary = t.mkColorOption "secondary";
        };
      };
    };

    colorProfile = lib.mkOption {
      type = lib.types.enum [
        "auto"
        "truecolor"
        "ansi256"
      ];
      default = defaults.colorProfile;
      description = ''
        Color depth for all prelude components:
        - `auto`: detect color depth from the terminal environment and output.
        - `truecolor`: force 24-bit color output.
        - `ansi256`: force quantization to the 256-color palette.
      '';
    };

    project = lib.mkOption {
      type = lib.types.str;
      default = defaults.project;
      description = "Project name shown in the motd banner and menu header.";
    };

    groups = lib.mkOption {
      type = lib.types.attrsOf t.groupType;
      default = defaults.groups;
      description = "Runnable task groups keyed by name for the interactive menu.";
      example = {
        develop = {
          order = 100;
          tasks.dev = {
            run = "pnpm dev";
            description = "start the dev server";
            key = "d";
          };
        };
      };
    };
  };
}
