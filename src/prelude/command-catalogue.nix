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
# Projections:
#   projectMenuGroups  → menu TUI JSON groups/tasks
#   projectMotdRows    → MOTD Getting Started { command, description }
#   projectMotdCatalog → flat catalogue used by motd.nix before row projection
{ lib }:
let
  normalizeArg = a: {
    token = a.token;
    description = a.description or "";
    required = a.required or false;
    boolean = a.boolean or false;
    options = a.options or [ ];
  };

  # Stable identity derived from the public command key. The first colon is
  # presentation-only (menu group + label); the complete key remains the
  # callable `x` name.
  commandIdentity =
    sourceName:
    let
      parts = lib.splitString ":" sourceName;
      grouped = builtins.length parts > 1;
      builtin = lib.elem sourceName [
        "menu"
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

  # Select commands for the MOTD Getting Started list.
  # - Commands with `motd` set appear at that sort order.
  # - `menu` is always included when present (opens the command palette).
  # - Most rows render as `x <name>`; `menu` is bare so it does not show the
  #   `x` prefix (it is a first-class PATH entrypoint, not an x dispatch).
  # Returns `{ name, command, description }` rows in display order.
  selectCommands =
    commands:
    let
      isMenu = entry: entry.name == "menu";
      motdOrder =
        entry:
        let
          order = entry.raw.motd or null;
        in
        # Menu shortcuts default ahead of project next-steps unless explicitly ordered.
        if order != null then
          order
        else if isMenu entry then
          0
        else
          null;
      motdEntries = lib.filter (entry: motdOrder entry != null) commands;
      sorted = lib.sort
        (
          a: b:
            let
              ao = motdOrder a;
              bo = motdOrder b;
            in
            if ao != bo then ao < bo else a.name < b.name
        )
        motdEntries;
    in
    map
      (entry: {
        name = entry.name;
        # Bare `menu` (no `x` prefix); every other MOTD row is an x dispatch.
        command = if isMenu entry then entry.name else entry.xInvocation;
        description = entry.description;
      })
      sorted;

  # --- projections -------------------------------------------------------------

  orEmpty = v: if v == null then "" else v;

  # Menu TUI JSON boundary: groups of tasks with the fields Go menu.Config
  # expects. Keeps catalogue metadata (usage/details/examples/args/key) intact.
  projectMenuGroups =
    groupOrder: commands:
    map
      (group: {
        title = group.title;
        tasks = map
          (t: {
            name = t.name;
            label = t.label;
            run = t.run;
            description = t.description;
            key = orEmpty t.key;
            usage = orEmpty t.usage;
            details = orEmpty t.details;
            examples = t.examples;
            args = t.args;
          })
          group.tasks;
      })
      (normalizeCommandGroups groupOrder commands);

  # Flat catalogue (normalized entries) used by motd.nix before row reduction.
  projectMotdCatalog =
    groupOrder: commands:
    flatCommands (normalizeCommandGroups groupOrder commands);

  # Reduced MOTD rows: only what the Go MOTD renderer paints.
  projectMotdRows =
    groupOrder: commands:
    map (row: { inherit (row) command description; }) (
      selectCommands (projectMotdCatalog groupOrder commands)
    );

in
{
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
