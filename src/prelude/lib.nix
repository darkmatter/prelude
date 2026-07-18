# Shared helpers for the prelude generators (motd.nix, menu.nix).
{ lib }:
let

  themes = import ./themes.nix;
  themeNames = lib.attrNames themes;

  # Resolve a theme name + per-token overrides into a concrete palette.
  # Overrides with null values fall through to the theme.
  resolvePalette =
    theme: overrides:
      assert lib.assertMsg
        (
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
    minHeight = spec.minHeight or 0;
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
    lib.sort
      (
        a: b:
        let
          aOrder = a.value.order or 1000;
          bOrder = b.value.order or 1000;
        in
        if aOrder != bOrder then aOrder < bOrder else a.name < b.name
      )
      (lib.mapAttrsToList lib.nameValuePair attrs);

  commandIdentity =
    sourceName:
    let
      parts = lib.splitString ":" sourceName;
      grouped = builtins.length parts > 1;
      builtin = lib.elem sourceName [
        "menu"
        "help"
        "docs"
      ];
      group =
        if builtin then
          "prelude"
        else if grouped then
          builtins.head parts
        else
          "develop";
      label = if grouped then lib.concatStringsSep ":" (lib.tail parts) else sourceName;
    in
    assert lib.assertMsg
      (
        builtins.match "[^ \t]+" sourceName != null
      ) "prelude: command key must be non-empty and contain no whitespace";
    assert lib.assertMsg
      (
        group != "" && label != ""
      ) "prelude: command key must have non-empty colon-separated segments";
    {
      inherit
        sourceName
        group
        label
        grouped
        ;
    };

  normalizeCommand =
    sourceName: command:
    let
      identity = commandIdentity sourceName;
    in
    identity
    // {
      # The key is both stable identity and public x command. The first colon
      # derives presentation only; it remains part of the key (`x go:test`).
      name = sourceName;
      # The Go menu still calls executable shell text `run` at its JSON boundary.
      run =
        let
          value = command.exec or null;
        in
        if value == null then identity.label else value;
      # Human-facing command text is independent from identity/group metadata.
      invocation =
        let
          value = command.invocation or null;
          run = command.exec or null;
        in
        if value != null then
          value
        else if run != null then
          run
        else
          identity.label;
      description = command.description or "";
      key = command.key or null;
      usage = command.usage or null;
      details = command.details or null;
      examples = command.examples or [ ];
      args = map normalizeArg (command.args or [ ]);
    };

  normalizeCommandEntries =
    commands:
    let
      baseEntries = map
        (
          { name, value }:
          (normalizeCommand name value)
          // {
            raw = value;
          }
        )
        (lib.mapAttrsToList lib.nameValuePair commands);
      invocations = map (entry: entry.invocation) baseEntries;
      duplicates = lib.filter
        (
          invocation: lib.count (candidate: candidate == invocation) invocations > 1
        )
        (lib.unique invocations);
      withDispatch = entry: entry // { xInvocation = "x ${lib.escapeShellArg entry.name}"; };
    in
    assert lib.assertMsg
      (
        duplicates == [ ]
      ) "prelude: duplicate canonical command invocation(s): ${lib.concatStringsSep ", " duplicates}";
    map withDispatch baseEntries;

  normalizeCommandGroups =
    groupOrder: commands:
    let
      entries = normalizeCommandEntries commands;
      availableGroups = lib.unique (map (entry: entry.group) entries);
      requestedGroups = [ "prelude" ] ++ groupOrder;
      preferredGroups = lib.unique (lib.filter (group: lib.elem group availableGroups) requestedGroups);
      remainingGroups = lib.sort builtins.lessThan (
        lib.filter (group: !lib.elem group preferredGroups) availableGroups
      );
      groupNames = preferredGroups ++ remainingGroups;
      commandsInGroup =
        group: lib.sort (a: b: a.label < b.label) (lib.filter (entry: entry.group == group) entries);
    in
    assert lib.assertMsg
      (
        lib.unique groupOrder == groupOrder
      ) "prelude: sort.groups must not contain duplicates";
    map
      (group: {
        title = group;
        tasks = commandsInGroup group;
      })
      groupNames;

  flatCommands = groups: lib.concatMap (group: group.tasks) groups;

  # Select commands whose `motd` sort order is set, sorted ascending by
  # `motd` then by command name. Returns `{ name, command, description }`
  # rows in display order. Commands with `motd = null` are hidden from the MOTD.
  selectCommands =
    commands:
    let
      motdEntries = lib.filter (entry: entry.raw.motd or null != null) commands;
      sorted = lib.sort
        (
          a: b:
            let
              ao = a.raw.motd;
              bo = b.raw.motd;
            in
            if ao != bo then ao < bo else a.name < b.name
        )
        motdEntries;
    in
    map
      (entry: {
        name = entry.name;
        command = entry.xInvocation;
        description = entry.description;
      })
      sorted;

  # Header status badges: order then key. Keep static text and/or live checks.
  normalizeHeaderStatus =
    status:
    builtins.filter (item: item.label != "" || item.status != "" || item.check != "") (
      map
        (
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
        )
        (sortOrderedAttrs status)
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
    map
      (
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
      )
      (sortOrderedAttrs recipes);

  # Navigation shortcuts are part of the component contract, not user data.
  # Deriving them from enablement keeps every rendered chip executable and
  # prevents configurations from hiding Prelude's built-in navigation.
  componentShortcuts =
    enabled:
    lib.optionals enabled.motd [
      {
        command = "motd";
        alias = "?";
      }
    ]
    ++ lib.optionals enabled.menu [
      {
        command = "menu";
        alias = "m";
      }
    ]
    ++ lib.optionals enabled.docs [
      {
        command = "docs";
        alias = "d";
      }
    ];

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
    commandIdentity
    normalizeCommand
    normalizeCommandEntries
    normalizeCommandGroups
    flatCommands
    selectCommands
    normalizeHeaderStatus
    normalizeRecipes
    componentShortcuts
    ;
}
