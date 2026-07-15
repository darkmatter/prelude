# Shared helpers for the prelude generators (motd.nix, menu.nix).
{ lib }:
let

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

  # Resolve a spacing spec into explicit sides. `x`/`y` are axis shorthands
  # (left+right / top+bottom); explicit sides supersede them.
  resolveSpacing = spec: {
    top = if (spec.top or null) != null then spec.top else (spec.y or 0);
    bottom = if (spec.bottom or null) != null then spec.bottom else (spec.y or 0);
    left = if (spec.left or null) != null then spec.left else (spec.x or 0);
    right = if (spec.right or null) != null then spec.right else (spec.x or 0);
  };

  textDefaults = {
    text = "";
    foreground = null;
    background = null;
    bold = false;
    italic = false;
    faint = false;
  };

  # Explicit colors override semantic palette roles at the Nix boundary so
  # renderers receive one normalized color rather than duplicating precedence.
  withRole =
    pal: role: text:
    text // { foreground = if text.foreground != null then text.foreground else pal.${role}; };

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
    commandNames: tasks:
    let
      tasksByName = lib.listToAttrs (map (task: lib.nameValuePair task.name task) tasks);
      missing = lib.filter (name: !(tasksByName ? ${name})) commandNames;
    in
    assert lib.assertMsg (
      missing == [ ]
    ) "prelude: motd.commands references unknown menu tasks: ${lib.concatStringsSep ", " missing}";
    map (name: {
      command = name;
      description = tasksByName.${name}.description;
    }) commandNames;

  # Header status badges: order then key. Keep static text and/or live checks.
  normalizeHeaderStatus =
    status:
    builtins.filter (item: item.label != "" || item.status != "" || item.check != "") (
      map (
        { value, ... }:
        {
          label = value.label or "";
          status = value.status or "";
          check = value.check or "";
          ok = value.ok or "ok";
          fail = value.fail or "fail";
          failLevel = value.failLevel or "error";
          output = value.output or "";
        }
      ) (sortOrderedAttrs status)
    );

  # Normalize a free-form recipe line into a step. Empty lines are dropped;
  # "#…" becomes a comment; everything else is a command.
  lineToStep =
    line:
    let
      trimmed = lib.removePrefix " " (lib.removeSuffix " " line);
    in
    if trimmed == "" then
      null
    else if lib.hasPrefix "#" trimmed then
      {
        command = "";
        comment =
          if lib.hasPrefix "# " trimmed then lib.removePrefix "# " trimmed else lib.removePrefix "#" trimmed;
      }
    else
      {
        command = trimmed;
        comment = "";
      };

  normalizeRecipeStep = step: {
    command = step.command or "";
    comment = step.comment or "";
  };

  normalizeRecipes =
    recipes:
    map (
      { name, value }:
      let
        title = value.title or null;
        explicitSteps = value.steps or [ ];
        legacyLines = value.lines or [ ];
        steps =
          if explicitSteps != [ ] then
            map normalizeRecipeStep explicitSteps
          else
            builtins.filter (s: s != null) (map lineToStep legacyLines);
      in
      {
        title = if title == null then name else title;
        inherit steps;
      }
    ) (sortOrderedAttrs recipes);

in
{
  inherit
    themes
    themeNames
    resolvePalette
    resolveSpacing
    textDefaults
    withRole
    sortOrderedAttrs
    normalizeArg
    normalizeTask
    normalizeGroups
    flatTasks
    normalizeCommands
    normalizeHeaderStatus
    normalizeRecipes
    ;
}
