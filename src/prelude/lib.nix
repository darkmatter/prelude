# Shared helpers for the prelude generators (motd.nix, menu.nix).
{ lib }:
let
  q = lib.escapeShellArg;

  themes = import ./themes.nix;
  themeNames = lib.attrNames themes;

  # Resolve a theme name + per-token overrides into a concrete palette.
  # Overrides with null values fall through to the theme.
  resolvePalette =
    theme: overrides:
    assert lib.assertMsg (
      themes ? ${theme}
    ) "prelude: unknown theme \"${theme}\" (expected one of: ${lib.concatStringsSep ", " themeNames})";
    themes.${theme} // lib.filterAttrs (_n: v: v != null) overrides;

  statusColor =
    pal: s:
    {
      success = pal.accent;
      error = pal.error;
      warning = pal.accent2;
      info = pal.dim;
    }
    .${s} or (throw "prelude: invalid status \"${s}\" (expected success|error|warning|info)");

  # gum resolves color depth at runtime from COLORTERM/TERM (sniffed on
  # stderr's tty). Our palettes are truecolor hex; without
  # COLORTERM=truecolor they get quantized to the 256-color palette, and
  # under tmux's default TERM=screen they can drop out entirely. These
  # exports only affect the generated script's own process.
  colorProfileSetup =
    profile:
    if profile == "truecolor" then
      [
        "export COLORTERM=truecolor"
        "case \"\${TERM:-}\" in *256color*) : ;; *) export TERM=xterm-256color ;; esac"
      ]
    else if profile == "ansi256" then
      [
        "export COLORTERM="
        "case \"\${TERM:-}\" in *256color*) : ;; *) export TERM=xterm-256color ;; esac"
      ]
    else
      assert lib.assertMsg (
        profile == "auto"
      ) "prelude: colorProfile must be auto, truecolor, or ansi256";
      [ ];

  # Resolve a spacing spec into explicit sides. `x`/`y` are axis shorthands
  # (left+right / top+bottom); explicit sides supersede them.
  resolveSpacing = spec: {
    top = if (spec.top or null) != null then spec.top else (spec.y or 0);
    bottom = if (spec.bottom or null) != null then spec.bottom else (spec.y or 0);
    left = if (spec.left or null) != null then spec.left else (spec.x or 0);
    right = if (spec.right or null) != null then spec.right else (spec.x or 0);
  };

  # gum/lipgloss CSS-style "top right bottom left" value.
  spacingTRBL = s: "${toString s.top} ${toString s.right} ${toString s.bottom} ${toString s.left}";

  textDefaults = {
    text = "";
    foreground = null;
    background = null;
    bold = false;
    italic = false;
    faint = false;
  };

  # Fall back to a palette role when no explicit foreground is set.
  withRole =
    pal: role: t:
    t // { foreground = if t.foreground != null then t.foreground else pal.${role}; };

  styleFlags =
    t:
    lib.concatStrings (
      lib.optional (t.foreground != null) " --foreground ${q (toString t.foreground)}"
      ++ lib.optional (t.background != null) " --background ${q (toString t.background)}"
      ++ lib.optional t.bold " --bold"
      ++ lib.optional t.italic " --italic"
      ++ lib.optional t.faint " --faint"
    );

  mkTextEl = name: t: "${name}=$(gum style${styleFlags t} ${q t.text})";

  # Runtime width setup: sets card_w (and inner_w when needsInner). "full"
  # widths track the terminal via tput, capped by maxWidth; fixed widths are
  # baked as plain assignments.
  #   sideCols: columns consumed outside card_w (margins + border)
  #   padCols:  total horizontal padding inside card_w (left + right)
  mkWidthSetup =
    {
      isFull,
      fixedWidth ? 70,
      maxWidth ? null,
      sideCols ? 0,
      padCols ? 0,
      needsInner ? false,
    }:
    lib.concatStringsSep "\n" (
      if isFull then
        [
          "term_w=$(tput cols 2>/dev/null || echo 80)"
          "card_w=$(( term_w - ${toString sideCols} ))"
        ]
        ++ lib.optional (
          maxWidth != null
        ) "if [ \"$card_w\" -gt ${toString maxWidth} ]; then card_w=${toString maxWidth}; fi"
        ++ [ "if [ \"$card_w\" -lt 10 ]; then card_w=10; fi" ]
        ++ lib.optionals needsInner [
          "inner_w=$(( card_w - ${toString padCols} ))"
          "if [ \"$inner_w\" -lt 1 ]; then inner_w=1; fi"
        ]
      else
        let
          w = if maxWidth != null then lib.min fixedWidth maxWidth else fixedWidth;
        in
        [ "card_w='${toString w}'" ]
        ++ lib.optional needsInner "inner_w='${toString (lib.max 1 (w - padCols))}'"
    );

  # --- task schema -------------------------------------------------------------

  normalizeArg = a: {
    token = a.token;
    description = a.description or "";
    required = a.required or false;
    boolean = a.boolean or false;
    options = a.options or [ ];
  };

  sortOrderedAttrs =
    attrs:
    lib.sort (
      a: b:
      let
        aOrder = a.value.order or 1000;
        bOrder = b.value.order or 1000;
      in
      if aOrder != bOrder then aOrder < bOrder else a.name < b.name
    ) (lib.mapAttrsToList lib.nameValuePair attrs);

  normalizeTask =
    name: task:
    assert lib.assertMsg (
      builtins.match "[^ \t]+" name != null
    ) "prelude: task name \"${name}\" must not contain whitespace";
    {
      inherit name;
      # Explicit null (e.g. from module option defaults) also falls back.
      run =
        let
          r = task.run or null;
        in
        if r == null then name else r;
      description = task.description or "";
      key = task.key or null;
      usage = task.usage or null;
      details = task.details or null;
      examples = task.examples or [ ];
      args = map normalizeArg (task.args or [ ]);
    };

  normalizeGroups =
    groups:
    let
      gs = map (
        { name, value }:
        let
          title = value.title or null;
        in
        {
          title = if title == null then name else title;
          tasks = map ({ name, value }: normalizeTask name value) (sortOrderedAttrs (value.tasks or { }));
        }
      ) (sortOrderedAttrs groups);
      names = lib.concatMap (group: map (task: task.name) group.tasks) gs;
    in
    assert lib.assertMsg (
      lib.unique names == names
    ) "prelude: task names must be unique across all groups";
    gs;

  flatTasks = groups: lib.concatMap (g: g.tasks) groups;

  normalizeCommands =
    commands:
    map (
      { value, ... }:
      {
        inherit (value) command;
        description = value.description or "";
      }
    ) (sortOrderedAttrs commands);

  normalizeRecipes =
    recipes:
    map (
      { name, value }:
      let
        title = value.title or null;
      in
      {
        title = if title == null then name else title;
        lines = value.lines or [ ];
      }
    ) (sortOrderedAttrs recipes);

in
{
  inherit
    q
    themes
    themeNames
    resolvePalette
    statusColor
    colorProfileSetup
    resolveSpacing
    spacingTRBL
    textDefaults
    withRole
    styleFlags
    mkTextEl
    mkWidthSetup
    normalizeArg
    normalizeTask
    normalizeGroups
    flatTasks
    normalizeCommands
    normalizeRecipes
    ;
}
