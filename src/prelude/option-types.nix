# Shared option types and builders for the prelude module options
# (src/prelude/options/*.nix).
{ lib }:
let
  # Colors are ANSI-256 numbers (e.g. 212) or hex strings (e.g. "#dddddd").
  colorType = lib.types.either lib.types.ints.unsigned lib.types.str;

  # Relative shade of a containing background: negative darkens, positive lightens.
  # Amount is in [ -1.0, 1.0 ]. Resolved at runtime against the terminal (card/
  # window) or the resolved card (nested description).
  relativeBgType = lib.types.addCheck (lib.types.submodule {
    options.relative = lib.mkOption {
      type = lib.types.either lib.types.float lib.types.int;
      description = "Signed shade amount relative to the containing background (−1..1).";
    };
  }) (v: (v.relative or 2) >= -1 && (v.relative or 2) <= 1);

  # Blend amount toward the theme background: 0 is the detected terminal
  # background, 1 is the theme `bg` token.
  blendBgType = lib.types.addCheck (lib.types.submodule {
    options.blend = lib.mkOption {
      type = lib.types.either lib.types.float lib.types.int;
      description = "Blend amount from the detected terminal background toward the theme background (0..1).";
    };
  }) (v: (v.blend or 2) >= 0 && (v.blend or 2) <= 1);

  # Terminal-relative backgrounds add `{ blend = n; }`; nested backgrounds only
  # support relative shading because their base may be the containing card.
  terminalBgType = lib.types.nullOr (
    lib.types.either lib.types.bool (
      lib.types.either colorType (lib.types.either relativeBgType blendBgType)
    )
  );

  # null/false = transparent, true = theme token, explicit color, or { relative }.
  bgType = lib.types.nullOr (
    lib.types.either lib.types.bool (lib.types.either colorType relativeBgType)
  );

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
            type = bgType;
            default = background;
            description = ''
              Background fill: null/false = inherit/transparent, true = theme bg,
              a color, or `{ relative = ±n; }` relative to the resolved card.
            '';
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
          tips = lib.mkOption {
            type = lib.types.listOf lib.types.str;
            default = [ ];
            description = "Optional tip lines under the body. Wrap commands in backticks for accent highlighting.";
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

  # Header status badge. Static: { label?, status }. Live: { label?, check, ok?, fail?, failLevel? }.
  # `check` is a shell command; exit 0 → success (ok text / stdout), else error
  # (fail) — or warning when failLevel = "warning".
  headerStatusType = lib.types.submodule {
    options = {
      order = lib.mkOption {
        type = lib.types.int;
        default = 1000;
        description = "Display order; attribute name breaks ties.";
      };
      label = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Dim label left of the status indicator.";
      };
      status = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Static status text (no check). Ignored when `check` is set.";
      };
      check = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Shell command that determines the badge. Exit 0 = success; non-zero = error. First stdout line may become the status text when non-empty.";
      };
      ok = lib.mkOption {
        type = lib.types.str;
        default = "ok";
        description = "Status text when `check` succeeds and stdout is empty.";
      };
      fail = lib.mkOption {
        type = lib.types.str;
        default = "fail";
        description = "Status text when `check` fails and stdout is empty.";
      };
      failLevel = lib.mkOption {
        type = lib.types.enum [
          "error"
          "warning"
        ];
        default = "error";
        description = "Severity when `check` fails: error dot (default) or accent2 warning dot.";
      };
      output = lib.mkOption {
        type = lib.types.enum [
          ""
          "light"
          "diagnostic"
        ];
        default = "";
        description = ''
          Controls what the badge shows after a check runs:
          - `""` (default): configured ok/fail text, or first output line.
          - `"light"`: colored dot + label only; discard text and diagnostics.
          - `"diagnostic"`: ok/fail text plus captured output rendered below the MOTD.
        '';
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
        description = ''
          Command the task executes; defaults to the task key. Unless `run`
          starts with the task's own name (meaning the command already exists
          on PATH), the module bundles a wrapper executable named after the
          task into `packages.menu` (delegating to `menu <name> …`), so every
          task the menu displays is directly invocable in the devshell.
        '';
      };
      runtimePackages = lib.mkOption {
        type = lib.types.listOf lib.types.package;
        default = [ ];
        internal = true;
        description = "Packages automatically bundled for this task by prelude.lib.mkTask.";
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

  recipeStepType = lib.types.submodule {
    options = {
      command = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Runnable command line. Mutually exclusive with comment in practice.";
      };
      comment = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Caption rendered as a # comment. Mutually exclusive with command in practice.";
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
      steps = lib.mkOption {
        type = lib.types.listOf recipeStepType;
        default = [ ];
        description = "Recipe steps: { command = \"...\"; } or { comment = \"...\"; }.";
      };
      # Legacy free-form lines; normalized into steps by lib.normalizeRecipes.
      lines = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
        description = "Legacy display lines (# comments / commands). Prefer steps.";
      };
    };
  };

  shortcutType = lib.types.submodule {
    options = {
      command = lib.mkOption {
        type = lib.types.str;
        description = "Command name shown in the shortcuts line.";
      };
      alias = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Optional short alias in parentheses.";
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
    relativeBgType
    blendBgType
    terminalBgType
    bgType
    widthType
    alignType
    mkTextOption
    mkColorOption
    mkSpacingOption
    headerStatusType
    exampleType
    argType
    taskType
    groupType
    recipeStepType
    recipeType
    shortcutType
    envItemType
    ;
}
