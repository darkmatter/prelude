# prelude.motd.* options — the static devshell welcome banner.
{ lib, ... }:
let
  defaults = import ../defaults.nix;
  t = import ../option-types.nix { inherit lib; };
in
{
  options.prelude.motd = {
    enable = lib.mkEnableOption "devshell MOTD banner";

    title = lib.mkOption {
      type = lib.types.nullOr lib.types.path;
      default = defaults.motd.title;
      description = "Checked-in multiline title file. Generate it from title.nix with the prelude-title utility; null uses the project-name header.";
      example = lib.literalExpression "./title.txt";
    };

    background = lib.mkOption {
      type = t.terminalBgType;
      default = defaults.motd.background;
      description = ''
        Block background fill: true uses the theme `bg` token, a color, or
        `{ relative = ±n; }` relative to the terminal background, or
        `{ blend = n; }` blending from the terminal toward theme `bg` (0..1).
        Falls back to windowBackground when unset. Detection failure uses theme bg.
      '';
      example = {
        relative = -0.05;
      };
    };

    windowBackground = lib.mkOption {
      type = t.terminalBgType;
      default = defaults.motd.windowBackground;
      description = ''
        Window background: paints the full terminal width (margins, gutters,
        line remainders). true uses theme `bg`, a color, `{ relative = ±n; }`,
        or `{ blend = n; }` from the terminal toward theme `bg` (0..1).
      '';
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
              "inline"
              "inverted"
            ];
            default = defaults.motd.header.titleStyle;
            description = "Wordmark treatment: plain / spine / bracketed / label / inline / inverted (accent chip).";
          };
          tagline = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.header.tagline;
            description = "Bold accent2 line under the header rule (e.g. \"Dev Shell Activated\").";
          };
          subtitle = lib.mkOption {
            type = lib.types.str;
            default = defaults.motd.header.subtitle;
            description = "Faint muted line under the tagline (e.g. \"Your environment is ready\").";
          };
          taglineLayout = lib.mkOption {
            type = lib.types.enum [
              "stack"
              "inline"
            ];
            default = defaults.motd.header.taglineLayout;
            description = "How tagline + subtitle are arranged: stack (two lines) or inline (one row, joined by ·).";
          };
          taglineAlign = lib.mkOption {
            type = lib.types.enum [
              "left"
              "center"
            ];
            default = defaults.motd.header.taglineAlign;
            description = "Horizontal alignment of the tagline/subtitle block within the content band.";
          };
          background = lib.mkOption {
            type = t.bgType;
            default = defaults.motd.header.background;
            description = ''
              Header bar fill: true = raised lightened bar (default), null/false =
              transparent (fg-only), a color, or `{ relative = ±n; }` vs terminal bg.
            '';
            example = null;
          };
          status = lib.mkOption {
            type = lib.types.attrsOf t.headerStatusType;
            default = defaults.motd.header.status;
            description = ''
              Keyed status badges on the header, sorted by `order` then name.

              Static: `{ label?, status }` — always shows that text.
              Live: `{ label?, check, ok?, fail?, failLevel? }` — runs `check`
              (shell) with a spinner; exit 0 paints a green/accent dot with `ok`
              (or stdout), non-zero paints an error dot with `fail`. Set
              `failLevel = "warning"` for a non-fatal accent2 dot instead.

              Empty attrs hide the status region. Tight rows drop labels.
            '';
            example = {
              nix = {
                order = 100;
                label = "nix";
                check = "nix --version >/dev/null";
                ok = "ready";
              };
              flake = {
                order = 200;
                label = "flake";
                check = "test -f flake.nix";
                ok = "ok";
                fail = "missing";
              };
            };
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
      type = lib.types.listOf lib.types.str;
      default = defaults.motd.commands;
      description = "Ordered menu task names rendered as runnable `$ command` rows with descriptions inherited from those tasks.";
      example = [
        "dev"
        "check"
      ];
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
