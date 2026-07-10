# Shared option types and builders for the prelude module options
# (src/prelude/options/*.nix).
{ lib }:
let
  # Colors are ANSI-256 numbers (e.g. 212) or hex strings (e.g. "#dddddd").
  colorType = lib.types.either lib.types.ints.unsigned lib.types.str;

  # null/false = transparent, true = theme token, or an explicit color.
  bgType = lib.types.nullOr (lib.types.either lib.types.bool colorType);

  widthType = lib.types.either lib.types.int (lib.types.enum [ "full" ]);

  alignType = lib.types.enum [
    "left"
    "center"
    "right"
  ];

  # Styled text item. Defaults are baked into the submodule fields so partial
  # settings (e.g. `prelude.motd.description.text = "..."`) keep the per-option
  # styling. A null foreground falls back to the theme's role color.
  mkTextOption =
    {
      text ? "",
      foreground ? null,
      background ? null,
      bold ? false,
      faint ? false,
      description,
    }:
    lib.mkOption {
      inherit description;
      default = { };
      type = lib.types.submodule {
        options = {
          text = lib.mkOption {
            type = lib.types.str;
            default = text;
            description = "Text content. An empty string hides the item.";
          };
          foreground = lib.mkOption {
            type = lib.types.nullOr colorType;
            default = foreground;
            description = "Foreground color; null uses the theme role color.";
          };
          background = lib.mkOption {
            type = lib.types.nullOr colorType;
            default = background;
            description = "Background color.";
          };
          bold = lib.mkOption {
            type = lib.types.bool;
            default = bold;
            description = "Render bold.";
          };
          italic = lib.mkOption {
            type = lib.types.bool;
            default = false;
            description = "Render italic.";
          };
          faint = lib.mkOption {
            type = lib.types.bool;
            default = faint;
            description = "Render faint.";
          };
        };
      };
    };

  mkColorOption =
    role:
    lib.mkOption {
      type = lib.types.nullOr colorType;
      default = null;
      description = "Override the theme's `${role}` token.";
    };

  # Spacing submodule: x/y axis shorthands plus explicit sides that
  # supersede them (CSS-style).
  mkSpacingOption =
    { spacingDefaults, description }:
    let
      mkSide =
        side: axis:
        lib.mkOption {
          type = lib.types.nullOr lib.types.ints.unsigned;
          default = spacingDefaults.${side};
          description = "${side} spacing; supersedes `${axis}` when set.";
        };
    in
    lib.mkOption {
      inherit description;
      default = { };
      type = lib.types.submodule {
        options = {
          x = lib.mkOption {
            type = lib.types.ints.unsigned;
            default = spacingDefaults.x;
            description = "Horizontal spacing (columns, left and right).";
          };
          y = lib.mkOption {
            type = lib.types.ints.unsigned;
            default = spacingDefaults.y;
            description = "Vertical spacing (lines, top and bottom).";
          };
          top = mkSide "top" "y";
          bottom = mkSide "bottom" "y";
          left = mkSide "left" "x";
          right = mkSide "right" "x";
        };
      };
    };

  statusItemType = lib.types.submodule {
    options = {
      text = lib.mkOption {
        type = lib.types.str;
        description = "Label shown next to the status dot.";
      };
      status = lib.mkOption {
        type = lib.types.enum [
          "success"
          "error"
          "warning"
          "info"
        ];
        default = "success";
        description = "Status level; controls the dot color.";
      };
    };
  };

  exampleType = lib.types.submodule {
    options = {
      title = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Description line for the command. Empty to omit.";
      };
      command = lib.mkOption {
        type = lib.types.str;
        description = "Command text to display.";
      };
    };
  };

  argType = lib.types.submodule {
    options = {
      token = lib.mkOption {
        type = lib.types.str;
        description = "Token as typed, e.g. \"name\", \"<id>\", \"--template\", \"--force\".";
      };
      description = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "What this argument does.";
      };
      required = lib.mkOption {
        type = lib.types.bool;
        default = false;
        description = "Re-prompt until a value is provided.";
      };
      boolean = lib.mkOption {
        type = lib.types.bool;
        default = false;
        description = "A flag that takes no value (confirm prompt).";
      };
      options = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
        description = "Suggested values, offered as choices.";
      };
    };
  };

  taskType = lib.types.submodule {
    options = {
      order = lib.mkOption {
        type = lib.types.int;
        default = 1000;
        description = "Display order within the group; task name breaks ties.";
      };
      run = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Command the task executes; defaults to the task key.";
      };
      description = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "One-line description.";
      };
      key = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Single-key accelerator (menu fast path: `menu <key>`).";
      };
      usage = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Usage form shown in the menu details, e.g. \"just dev --port 3000\".";
      };
      details = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Extended description shown before arg entry.";
      };
      examples = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
        description = "Worked example invocations.";
      };
      args = lib.mkOption {
        type = lib.types.listOf argType;
        default = [ ];
        description = "Arguments/flags; presence triggers arg-entry mode in the menu.";
      };
    };
  };

  groupType = lib.types.submodule {
    options = {
      order = lib.mkOption {
        type = lib.types.int;
        default = 1000;
        description = "Display order; group name breaks ties.";
      };
      title = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Displayed group heading; defaults to the group name.";
      };
      tasks = lib.mkOption {
        type = lib.types.attrsOf taskType;
        default = { };
        description = "Tasks keyed by invocation name.";
      };
    };
  };

  commandType = lib.types.submodule {
    options = {
      order = lib.mkOption {
        type = lib.types.int;
        default = 1000;
        description = "Display order; command key breaks ties.";
      };
      command = lib.mkOption {
        type = lib.types.str;
        description = "Exact runnable command to display.";
      };
      description = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Short explanation of when to run the command.";
      };
    };
  };

  recipeType = lib.types.submodule {
    options = {
      order = lib.mkOption {
        type = lib.types.int;
        default = 1000;
        description = "Display order; recipe name breaks ties.";
      };
      title = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Displayed recipe title; defaults to the recipe name.";
      };
      lines = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
        description = "Display lines: empty lines add space, # lines are comments, and other lines are numbered commands.";
      };
    };
  };

  # Env chip: static (`value`) or resolved at render time (`probe`) —
  # exactly one of the two.
  envItemType = lib.types.submodule {
    options = {
      label = lib.mkOption {
        type = lib.types.str;
        description = "Chip label, e.g. \"node\".";
      };
      value = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Static chip value, e.g. \"22.3.0\".";
      };
      probe = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Probe command; its first output line becomes the value. Skipped on failure.";
      };
    };
  };
in
{
  inherit
    colorType
    bgType
    widthType
    alignType
    mkTextOption
    mkColorOption
    mkSpacingOption
    statusItemType
    exampleType
    argType
    taskType
    groupType
    commandType
    recipeType
    envItemType
    ;
}
