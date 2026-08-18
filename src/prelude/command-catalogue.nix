# Command catalogue domain module.
#
# Owns identity, normalization, grouping, selection, and surface projections
# for `prelude.commands`. Generators (menu.nix, motd.nix, module.nix) consume
# the domain and its projections rather than re-implementing catalogue rules.
#
# Domain entry shape (after normalizeCommandEntries):
#   { name, group, label, grouped, run, invocation, xInvocation,
#     description, key, usage, details, examples, args, raw }
#
# `group` is either the explicit override from `command.group` or the
# colon-inferred default; `grouped` tracks whether the key has a colon
# (PATH-wrapper vs x-only dispatch) and is independent of `group`.
#
# Projections:
#   projectMenuGroups  → menu TUI JSON groups/tasks
#   projectMotdRows    → MOTD Getting Started { command, description }
#   projectMotdCatalog → flat catalogue used by motd.nix before row projection
{lib}: let
  normalizeArg = a: {
    token = a.token;
    description = a.description or "";
    required = a.required or false;
    boolean = a.boolean or false;
    options = a.options or [];
  };

  # Stable identity derived from the public command key. The first colon is
  # presentation-only (menu group + label); the complete key remains the
  # callable `x` name. When `explicitGroup` is non-null it overrides the
  # colon-inferred group, letting callers place a flat key under a named group
  # without colon-prefixing it.
  commandIdentity = sourceName: explicitGroup: let
    parts = lib.splitString ":" sourceName;
    grouped = builtins.length parts > 1;
    builtin = lib.elem sourceName [
      "x"
      "docs"
    ];
    inferredGroup =
      if builtin
      then "prelude"
      else if grouped
      then builtins.head parts
      else "develop";
    group =
      if explicitGroup != null
      then explicitGroup
      else inferredGroup;
    label =
      if grouped
      then lib.concatStringsSep ":" (lib.tail parts)
      else sourceName;
  in
    assert lib.assertMsg (
      builtins.match "[^ \t]+" sourceName != null
    ) "prelude: command key must be non-empty and contain no whitespace";
    assert lib.assertMsg (
      group != "" && label != ""
    ) "prelude: command key must have non-empty colon-separated segments"; {
      inherit
        sourceName
        group
        label
        grouped
        ;
    };

  # Built-in Prelude entrypoints that have their own store-path binaries.
  # Commands named x/docs/motd with no explicit exec, or with an exec equal
  # to their name, are the surface binaries — not user commands that happen to
  # share the name. Legacy `menu` is only a PATH compatibility wrapper.
  builtinSurface = name: exec:
    if name == "x" && (exec == null || exec == "x")
    then "x"
    else if name == "docs" && (exec == null || exec == "docs")
    then "docs"
    else if name == "motd" && (exec == null || exec == "motd")
    then "motd"
    else null;

  normalizeCommand = sourceName: command: let
    identity = commandIdentity sourceName (command.group or null);
    exec = command.exec or null;
  in
    identity
    // {
      # The key is both stable identity and public x command. The first colon
      # derives presentation only; it remains part of the key (`x go:test`).
      name = sourceName;
      # The Go menu still calls executable shell text `run` at its JSON boundary.
      run =
        if exec == null
        then identity.label
        else exec;
      # Human-facing command text is independent from identity/group metadata.
      invocation = let
        value = command.invocation or null;
      in
        if value != null
        then value
        else if exec != null
        then exec
        else identity.label;
      description = command.description or "";
      key = command.key or null;
      usage = command.usage or null;
      details = command.details or null;
      examples = command.examples or [];
      args = map normalizeArg (command.args or []);
      builtinSurface = builtinSurface sourceName exec;
    };

  normalizeCommandEntries = commands: let
    baseEntries = map (
      {
        name,
        value,
      }:
        (normalizeCommand name value)
        // {
          raw = value;
        }
    ) (lib.mapAttrsToList lib.nameValuePair commands);
    invocations = map (entry: entry.invocation) baseEntries;
    duplicates = lib.filter (
      invocation: lib.count (candidate: candidate == invocation) invocations > 1
    ) (lib.unique invocations);
    withDispatch = entry: entry // {xInvocation = "x ${lib.escapeShellArg entry.name}";};
  in
    assert lib.assertMsg (
      duplicates == []
    ) "prelude: duplicate canonical command invocation(s): ${lib.concatStringsSep ", " duplicates}";
      map withDispatch baseEntries;

  normalizeCommandGroups = groupOrder: commands: let
    entries = normalizeCommandEntries commands;
    availableGroups = lib.unique (map (entry: entry.group) entries);
    requestedGroups = ["prelude"] ++ groupOrder;
    preferredGroups = lib.unique (lib.filter (group: lib.elem group availableGroups) requestedGroups);
    remainingGroups = lib.sort builtins.lessThan (
      lib.filter (group: !lib.elem group preferredGroups) availableGroups
    );
    groupNames = preferredGroups ++ remainingGroups;
    commandsInGroup = group: lib.sort (a: b: a.label < b.label) (lib.filter (entry: entry.group == group) entries);
  in
    assert lib.assertMsg (
      lib.unique groupOrder == groupOrder
    ) "prelude: sort.groups must not contain duplicates";
      map (group: {
        title = group;
        tasks = commandsInGroup group;
      })
      groupNames;

  flatCommands = groups: lib.concatMap (group: group.tasks) groups;

  # Select commands for the MOTD Getting Started list.
  # - Commands with `motd` set appear at that sort order.
  # - `x` is always included when present (opens the command palette).
  # - Ungrouped commands render bare: each one is on PATH (generated wrapper
  #   or first-class entrypoint), so the row matches what the user types. The
  #   `x` dispatcher remains the fallback when another command shadows them.
  # - Grouped commands (`go:test`) have no PATH entry — the complete key is
  #   only callable through `x`, so those rows keep the `x` dispatch form.
  # Returns `{ name, command, description }` rows in display order.
  selectCommands = commands: let
    isPalette = entry: entry.name == "x";
    motdOrder = entry: let
      order = entry.raw.motd or null;
    in
      # Palette entry defaults ahead of project next-steps unless explicitly ordered.
      if order != null
      then order
      else if isPalette entry
      then 0
      else null;
    motdEntries = lib.filter (entry: motdOrder entry != null) commands;
    sorted =
      lib.sort (
        a: b: let
          ao = motdOrder a;
          bo = motdOrder b;
        in
          if ao != bo
          then ao < bo
          else a.name < b.name
      )
      motdEntries;
  in
    map (entry: {
      name = entry.name;
      # Ungrouped commands run bare from PATH; grouped keys only run
      # through the `x` dispatcher, so they keep the dispatch form.
      command =
        if entry.grouped
        then entry.xInvocation
        else entry.name;
      description = entry.description;
    })
    sorted;

  # --- projections -------------------------------------------------------------

  orEmpty = v:
    if v == null
    then ""
    else v;

  # Menu TUI JSON boundary: groups of tasks with the fields Go menu.Config
  # expects. Keeps catalogue metadata (usage/details/examples/args/key) intact.
  # `command` is the user-runnable form: ungrouped entries are PATH commands,
  # while colon-grouped catalogue identities dispatch through `x`. Consumers
  # such as the shell status host must use this rather than reconstructing
  # invocation rules from a display label.
  projectMenuGroups = groupOrder: commands:
    map (group: {
      title = group.title;
      tasks =
        map (t: {
          name = t.name;
          label = t.label;
          run = t.run;
          command =
            if t.grouped
            then t.xInvocation
            else t.name;
          description = t.description;
          key = orEmpty t.key;
          usage = orEmpty t.usage;
          details = orEmpty t.details;
          examples = t.examples;
          args = t.args;
        })
        group.tasks;
    }) (normalizeCommandGroups groupOrder commands);

  # Flat catalogue (normalized entries) used by motd.nix before row reduction.
  projectMotdCatalog = groupOrder: commands: flatCommands (normalizeCommandGroups groupOrder commands);

  # Reduced MOTD rows: only what the Go MOTD renderer paints.
  projectMotdRows = groupOrder: commands:
    map (row: {inherit (row) command description;}) (
      selectCommands (projectMotdCatalog groupOrder commands)
    );
in {
  inherit
    normalizeArg
    commandIdentity
    normalizeCommand
    normalizeCommandEntries
    normalizeCommandGroups
    flatCommands
    selectCommands
    projectMenuGroups
    projectMotdCatalog
    projectMotdRows
    ;
}
