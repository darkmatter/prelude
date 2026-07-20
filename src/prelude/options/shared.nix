# Top-level prelude.* options: shared visual/project identity plus the command
# catalogue used by the interactive menu and MOTD.
{ lib, ... }:
let
  plib = import ../lib.nix { inherit lib; };
  defaults = import ../defaults.nix;
  t = import ../option-types.nix { inherit lib; };
in
{
  options = {
    prelude = {
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
            success = t.mkColorOption "success";
            warning = t.mkColorOption "warning";
            info = t.mkColorOption "info";
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

      commands = lib.mkOption {
        type = lib.types.attrsOf t.commandType;
        default = defaults.commands;
        description = "Project commands keyed by their public `x` name. The first colon infers the menu group; the remaining suffix is the displayed name, while the complete key remains callable.";
        example = {
          dev = {
            description = "start the dev server";
            exec = "pnpm dev";
          };
          "database:migrate" = {
            description = "apply pending migrations";
            exec = "drizzle-kit migrate";

          };
        };
      };

      sort.groups = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = defaults.sort.groups;
        description = "Preferred command-group order. Groups omitted from this list follow alphabetically; Prelude's own group remains first.";
        example = [
          "develop"
          "database"
          "deploy"
        ];
      };
    };
  };
}
