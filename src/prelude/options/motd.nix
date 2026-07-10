# prelude.motd.* options — the static devshell welcome banner.
{ lib, ... }:
let
  defaults = import ../defaults.nix;
  t = import ../option-types.nix { inherit lib; };
in
{
  options.prelude.motd = {
    enable = lib.mkEnableOption "devshell MOTD banner";

    background = lib.mkOption {
      type = t.bgType;
      default = defaults.motd.background;
      description = "Block background fill: true uses the theme's `bg` token, or pass a color. Falls back to windowBackground when unset.";
    };

    windowBackground = lib.mkOption {
      type = t.bgType;
      default = defaults.motd.windowBackground;
      description = "Window background: paints the full terminal width (margins, alignment gutters, line remainders). true uses the theme's `bg` token, or pass a color.";
    };

    clearScreen = lib.mkOption {
      type = lib.types.bool;
      default = defaults.motd.clearScreen;
      description = "Clear the terminal before rendering the motd.";
    };

    margin = t.mkSpacingOption {
      spacingDefaults = defaults.motd.margin;
      description = "Margin around the motd block: top/bottom blank lines, left/right offset columns. Sides supersede the x/y axes.";
    };

    align = lib.mkOption {
      type = t.alignType;
      default = defaults.motd.align;
      description = "Horizontal placement of the motd block against the terminal window (content inside stays left-aligned).";
    };

    loadLine = lib.mkOption {
      type = lib.types.str;
      default = defaults.motd.loadLine;
      description = "Dim load line above the banner (a themed ✓ is appended). Empty disables.";
    };

    banner = lib.mkOption {
      default = { };
      description = "Project banner box.";
      type = lib.types.submodule {
        options = {
          badge = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.banner.badge;
            description = "Glyph before the project name.";
          };
          label = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.banner.label;
            description = "Label after the project name, e.g. \"development shell\".";
          };
          tagline = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.banner.tagline;
            description = "Tagline shown beneath the project name.";
          };
          border = lib.mkOption {
            default = { };
            description = "Banner box border.";
            type = lib.types.submodule {
              options = {
                width = lib.mkOption {
                  type = lib.types.ints.unsigned;
                  default = defaults.motd.banner.border.width;
                  description = "Border width: 0 hides the border, 1 is a normal border, 2+ renders thick.";
                };
                foreground = lib.mkOption {
                  type = lib.types.nullOr t.colorType;
                  default = defaults.motd.banner.border.foreground;
                  description = "Border color; null uses the theme's accentBorder token.";
                };
                rounded = lib.mkOption {
                  type = lib.types.bool;
                  default = defaults.motd.banner.border.rounded;
                  description = "Round the corners (applies at width 1).";
                };
              };
            };
          };
          statusItems = lib.mkOption {
            type = lib.types.listOf t.statusItemType;
            default = defaults.motd.banner.statusItems;
            description = "Status indicator chips rendered right-aligned inside the banner.";
            example = [
              {
                text = "devshell";
                status = "success";
              }
            ];
          };
        };
      };
    };

    description = t.mkTextOption (
      defaults.motd.description
      // {
        description = "Styled text rendered beneath the banner (theme fg role). Empty text hides it.";
      }
    );

    env = lib.mkOption {
      type = lib.types.listOf t.envItemType;
      default = defaults.motd.env;
      description = "Env info chips, rendered in order. Each item sets exactly one of `value` (static) or `probe` (command whose first output line becomes the value; skipped on failure).";
      example = [
        {
          label = "node";
          value = "22.3.0";
        }
        {
          label = "nix";
          probe = "nix --version | awk '{print $NF}'";
        }
      ];
    };

    commands = lib.mkOption {
      type = lib.types.attrsOf t.commandType;
      default = defaults.motd.commands;
      description = "Primary next-step commands keyed by identity. Each row renders the exact runnable command and an optional description.";
      example = {
        check = {
          order = 100;
          command = "nix flake check";
          description = "verify the flake";
        };
      };
    };

    recipes = lib.mkOption {
      type = lib.types.attrsOf t.recipeType;
      default = defaults.motd.recipes;
      description = "Multi-step workflows keyed by name. Empty lines add space, # lines are comments, and other lines are numbered commands.";
      example.clean-local-stack = {
        title = "spin up a clean local stack";
        lines = [
          "# Start backing services"
          "just db:up"
          ""
          "just db:migrate && just db:seed"
          "just dev"
        ];
      };
    };

    git = lib.mkOption {
      type = lib.types.bool;
      default = defaults.motd.git;
      description = "Show a git segment (branch, ahead, dirty) when inside a repo.";
    };

    footer = lib.mkOption {
      type = lib.types.bool;
      default = defaults.motd.footer;
      description = "Show the inverted footer bar.";
    };

    footerHint = lib.mkOption {
      type = lib.types.str;
      default = defaults.motd.footerHint;
      description = "Right-aligned hint text in the footer bar.";
    };

    width = lib.mkOption {
      type = t.widthType;
      default = defaults.motd.width;
      description = "Content width, or \"full\" to fill the terminal width.";
    };

    maxWidth = lib.mkOption {
      type = lib.types.nullOr lib.types.ints.unsigned;
      default = defaults.motd.maxWidth;
      description = "Maximum content width.";
    };
  };
}
