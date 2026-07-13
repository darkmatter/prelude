# Shared defaults for the prelude components (motd, menu).
#
# Single source of truth consumed by both the module options
# (src/prelude/options/*.nix) and the generators (fallbacks for direct
# mkMotd/mkMenu consumers).
#
# Text items intentionally carry no colors here — when `foreground` is null
# the generators fall back to the palette role for that element, so the
# selected theme drives the look. Explicit colors always win.
{
  # Theme name (see themes.nix) + per-token palette overrides.
  theme = "phosphor";
  palette = { };

  # Color depth: "auto" detects the terminal profile, "truecolor" forces
  # 24-bit output, and "ansi256" forces quantization to 256 colors.
  colorProfile = "auto";

  # Project identity, shared by motd and menu.
  project = "devshell";

  # Runnable groups and tasks keyed by name for the interactive menu.
  # Both levels accept `order ? 1000`; names break equal-order ties.
  # Task: { run ? taskKey, description ? "", key ? null, usage ? null,
  #         details ? null, examples ? [ ],
  #         args ? [ { token, description ? "", required ? false,
  #                    boolean ? false, options ? [ ] } ] }
  groups = { };

  # --- motd --------------------------------------------------------------------
  # Layout matches src/motd-playground: header bar → description → env →
  # Getting Started (commands + examples) → shortcuts.

  motd = {
    # Block background fill: null/false = transparent, true = theme `bg`
    # token, or an explicit color. Falls back to windowBackground when
    # unset.
    background = null;

    # Window background: paints the full terminal width — margins,
    # alignment gutters, and line remainders. null/false = transparent,
    # true = theme `bg` token, or an explicit color.
    windowBackground = null;

    # Clear the terminal before rendering — with the top margin below this
    # renders as a clean greeting screen on shell entry.
    clearScreen = true;

    # Margin around the block: top/bottom blank lines, left/right columns
    # folded into the horizontal offset. x/y axes work as shorthands;
    # explicit sides supersede them.
    margin = {
      x = 0;
      y = 0;
      top = 10;
      bottom = null;
      left = null;
      right = null;
    };

    # Horizontal placement of the motd block against the terminal window
    # (content inside stays left-aligned).
    align = "center";

    # Inner padding between content and the block edge. Header and
    # shortcuts stay edge-to-edge; only the middle sections are inset.
    padding = {
      x = 0;
      y = 0;
      top = null;
      bottom = null;
      left = null;
      right = null;
    };

    # Filled hero bar: wordmark variant + status + tagline beneath.
    header = {
      # plain | spine | bracketed | label
      titleStyle = "spine";
      tagline = "everything you need to build, test & ship";
      statusLabel = "";
      statusLabelCompact = "";
      statusText = "ready";
    };

    # Styled text rendered beneath the header (theme fg role). An empty
    # text hides it. { text, foreground, background, bold, italic, faint }
    description = {
      text = "";
    };

    # Env info chips, rendered in order. Each item sets exactly one of:
    #   { label; value; }  — static chip
    #   { label; probe; }  — command run at render time; first output line
    #                        becomes the value, skipped on failure
    env = [ ];

    # Primary runnable commands shown with dotted leaders.
    # The module adds menu/menu-list defaults when the menu is enabled;
    # direct mkMotd consumers opt in explicitly.
    commands = { };

    # Multi-step workflows keyed by name. Prefer `steps`; legacy `lines`
    # are normalized into steps at the Nix boundary.
    recipes = { };

    # Show a git segment (branch, ahead, dirty) when inside a repo.
    git = true;

    # Unified commands + examples region labels.
    gettingStarted = {
      heading = "Getting Started";
      commandsLabel = "commands";
      examplesLabel = "examples";
    };

    # Quiet right-aligned discoverability chips (replaces the footer bar).
    # Item: { command; alias ? ""; }
    shortcuts = [ ];

    width = "full";
    maxWidth = 96;
  };

  # --- menu --------------------------------------------------------------------

  menu = {
    # Placeholder in the filter input.
    placeholder = "type to filter commands…";

    # Filter list height (rows).
    height = 12;

    # Execute the selected task (exec bash -c). When false, print it instead.
    execute = true;

    width = "full";
    maxWidth = 96;
  };
}
