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

  # --- command schema ----------------------------------------------------------

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

  normalizeCommand =
    name: command:
    assert lib.assertMsg (
      builtins.match "[^ \t]+" name != null
    ) "prelude: command name \"${name}\" must not contain whitespace";
    {
      inherit name;
      # The Go menu still calls this field `run` at its JSON boundary.
      run =
        let
          value = command.exec or null;
        in
        if value == null then name else value;
      description = command.description or "";
      key = command.key or null;
      usage = command.usage or null;
      details = command.details or null;
      examples = command.examples or [ ];
      args = map normalizeArg (command.args or [ ]);
    };

  normalizeCommandGroups =
    commands:
    let
      ordered = map (
        { name, value }:
        {
          group = value.group or "general";
          command = normalizeCommand name value;
        }
      ) (sortOrderedAttrs commands);
      groupNames = lib.unique (map (item: item.group) ordered);
    in
    map (group: {
      title = group;
      tasks = map (item: item.command) (lib.filter (item: item.group == group) ordered);
    }) groupNames;

  flatCommands = groups: lib.concatMap (group: group.tasks) groups;

  selectCommands =
    commandNames: commands:
    let
      commandsByName = lib.listToAttrs (map (command: lib.nameValuePair command.name command) commands);
      missing = lib.filter (name: !(commandsByName ? ${name})) commandNames;
    in
    assert lib.assertMsg (
      missing == [ ]
    ) "prelude: motd.commands references unknown commands: ${lib.concatStringsSep ", " missing}";
    map (name: {
      command = name;
      description = commandsByName.${name}.description;
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
          async = value.async or true;
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
    normalizeCommand
    normalizeCommandGroups
    flatCommands
    selectCommands
    normalizeHeaderStatus
    normalizeRecipes
    ;
}
