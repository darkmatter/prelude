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

    padding = t.mkSpacingOption {
      spacingDefaults = defaults.motd.padding;
      description = "Inner padding between content and the block edge. The header and shortcuts stay edge-to-edge; only middle sections are inset. Sides supersede the x/y axes.";
    };

    header = lib.mkOption {
      default = { };
      description = "Filled hero bar: wordmark variant, status, and tagline beneath.";
      type = lib.types.submodule {
        options = {
          titleStyle = lib.mkOption {
            type = lib.types.enum [
              "plain"
              "spine"
              "bracketed"
              "label"
            ];
            default = defaults.motd.header.titleStyle;
            description = "Wordmark treatment on the header bar.";
          };
          tagline = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.header.tagline;
            description = "Tagline shown beneath the header bar on the page background.";
          };
          statusLabel = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.header.statusLabel;
            description = "Dim label left of the status dot (e.g. \"nix develop · flake @ abc\").";
          };
          statusLabelCompact = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.header.statusLabelCompact;
            description = "Shorter status label used when the full label does not fit.";
          };
          statusText = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.header.statusText;
            description = "Muted text after the status dot (e.g. \"ready\"). Empty hides the status.";
          };
        };
      };
    };

    description = t.mkTextOption (
      defaults.motd.description
      // {
        description = "Styled text rendered beneath the header (theme fg role). Empty text hides it.";
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
      description = "Primary next-step commands keyed by identity. Each row renders `$ command` with dotted leaders to the description.";
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
      description = "Multi-step workflows keyed by name. Prefer `steps` ({ command } | { comment }); legacy `lines` are normalized into steps.";
      example.clean-local-stack = {
        title = "spin up a clean local stack";
        steps = [
          { comment = "start postgres + redis first"; }
          { command = "just db:up"; }
          { command = "just db:migrate && just db:seed"; }
          { command = "just dev"; }
        ];
      };
    };

    git = lib.mkOption {
      type = lib.types.bool;
      default = defaults.motd.git;
      description = "Show a git segment (branch, ahead, dirty) when inside a repo.";
    };

    gettingStarted = lib.mkOption {
      default = { };
      description = "Labels for the unified commands + examples region.";
      type = lib.types.submodule {
        options = {
          heading = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.gettingStarted.heading;
            description = "Centered heading above the commands/examples groups.";
          };
          commandsLabel = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.gettingStarted.commandsLabel;
            description = "Dim sub-label above the commands list.";
          };
          examplesLabel = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.gettingStarted.examplesLabel;
            description = "Dim sub-label above the recipe codeblocks.";
          };
        };
      };
    };

    shortcuts = lib.mkOption {
      type = lib.types.listOf t.shortcutType;
      default = defaults.motd.shortcuts;
      description = "Right-aligned discoverability chips that close the composition (replaces the footer bar).";
      example = [
        {
          command = "menu";
          alias = "m";
        }
      ];
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

    fullscreen = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Fill the entire terminal width with no cap. Equivalent to width = \"full\" and maxWidth = null.";
    };
  };
}
