# Command catalogue domain module.
#
# Owns identity, normalization, the subcommand tree, grouping, selection, and
# surface projections for `prelude.commands`. Generators (menu.nix, motd.nix,
# packages.nix) consume the domain and its projections rather than
# re-implementing catalogue rules. ts/src/internal/catalogue.ts ports these
# rules; the conformance fixture holds the two to the same output.
#
# A key is words separated by one space or `/`: `"db migrate"` and
# `"db/migrate"` both name `migrate` under `db`, run as `x db migrate`. `:` is
# an ordinary name character. Groups are never parsed from keys: a command's
# group is its explicit `group`, else "" (listed above every heading).
#
# Entry shape (after normalizeCommandEntries):
#   { sourceName, path, name, label, group, root, direct, synthesized, exec,
#     run, invocation, xInvocation, description, key, usage, details,
#     examples, args, builtinSurface, raw }
#
# Node shape (commandNodes): an entry, or a synthesized one for a path only
# its subcommands declare, plus { children, container, onPath, command }. A
# container has children and no `exec` of its own: it only opens them
# (run = ""). A top-level command is on PATH when it is declared and its name
# has no `:`; its whole subtree then runs bare (`db migrate`), else through
# `x`. Synthesized parents stay off PATH, so `"go test"` cannot shadow `go`.
#
# Projections:
#   projectMenuGroups  → menu TUI JSON groups/tasks (children nested)
#   projectMotdRows    → MOTD Getting Started { command, description }
#   projectMotdCatalog → every node, flattened, before row projection
{lib}: let
  normalizeArg = a: {
    token = a.token;
    description = a.description or "";
    required = a.required or false;
    boolean = a.boolean or false;
    options = a.options or [];
    default = a.default or null;
  };

  keyPattern = "[A-Za-z0-9:_.-]+([ /][A-Za-z0-9:_.-]+)*";

  # A key's words: one space or `/` separates a subcommand from its parent.
  keyPath = sourceName:
    assert lib.assertMsg (
      builtins.match keyPattern sourceName != null
    ) "prelude: command key \"${sourceName}\" must be words of letters, digits, : _ . - separated by a single space or /";
      lib.splitString " " (lib.replaceStrings ["/"] [" "] sourceName);

  # `x <words>`, each word shell-quoted only when it needs it.
  xInvocationOf = path: "x ${lib.concatMapStringsSep " " lib.escapeShellArg path}";

  # Identity from the public key. `direct` says the root's name may be a PATH
  # command: roots containing `:` are reachable only through `x`.
  identityOf = sourceName: explicitGroup: path: let
    name = lib.concatStringsSep " " path;
    root = builtins.head path;
    subcommand = lib.length path > 1;
    group =
      if explicitGroup != null
      then explicitGroup
      else if !subcommand && lib.elem name ["x" "docs"]
      then "prelude"
      else "";
    direct = !lib.hasInfix ":" root;
  in
    assert lib.assertMsg (!(subcommand && explicitGroup != null)) "prelude: \"${name}\" is a subcommand of \"${root}\"; set group on \"${root}\" instead"; {
      inherit sourceName path name root group direct;
      label = lib.last path;
      xInvocation = xInvocationOf path;
      synthesized = false;
    };

  commandIdentity = sourceName: explicitGroup: identityOf sourceName explicitGroup (keyPath sourceName);

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
      inherit exec;
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
      builtinSurface = builtinSurface identity.name exec;
    };

  normalizeCommandEntries = commands: let
    entries = map (
      {
        name,
        value,
      }:
        (normalizeCommand name value)
        // {
          raw = value;
        }
    ) (lib.mapAttrsToList lib.nameValuePair commands);
    # `db/migrate` and `db migrate` are one command; declaring both is a typo.
    sameName = lib.filter (group: lib.length group > 1) (
      lib.attrValues (lib.groupBy (entry: entry.name) entries)
    );
  in
    assert lib.assertMsg (sameName == []) (
      "prelude: these keys name the same command: "
      + lib.concatMapStringsSep "; " (group: lib.concatMapStringsSep ", " (entry: "\"${entry.sourceName}\"") group) sameName
    ); entries;

  # Every runnable command keeps one canonical invocation. Containers run
  # nothing, so two exec-less parents (`db`, `api db`) never clash.
  checkInvocations = nodes: let
    invocations = map (node: node.invocation) (lib.filter (node: !node.container && !node.synthesized) nodes);
    duplicates = lib.filter (
      invocation: lib.count (candidate: candidate == invocation) invocations > 1
    ) (lib.unique invocations);
  in
    lib.assertMsg (
      duplicates == []
    ) "prelude: duplicate canonical command invocation(s): ${lib.concatStringsSep ", " duplicates}";

  # A node for a path only subcommands declare: the parent the menu needs to
  # show them under, with no behavior of its own.
  synthesizedNode = path:
    identityOf (lib.concatStringsSep " " path) null path
    // {
      synthesized = true;
      exec = null;
      run = "";
      invocation = "";
      description = "";
      key = null;
      usage = null;
      details = null;
      examples = [];
      args = [];
      builtinSurface = null;
      raw = {};
    };

  byLabel = a: b:
    if a.label != b.label
    then a.label < b.label
    else a.name < b.name;

  # The nodes one level below a shared path, at word `depth`: the declared
  # command for each next word, or a synthesized parent, with its own
  # subcommands below it.
  buildNodes = depth: entries: let
    word = entry: builtins.elemAt entry.path depth;
    node = segment: let
      members = lib.filter (entry: word entry == segment) entries;
      declared = lib.findFirst (entry: lib.length entry.path == depth + 1) null members;
      children = buildNodes (depth + 1) (lib.filter (entry: lib.length entry.path > depth + 1) members);
      base =
        if declared != null
        then declared
        else synthesizedNode (lib.take (depth + 1) (builtins.head members).path);
      count = lib.length children;
      container = children != [] && base.exec == null;
    in
      base
      // {
        inherit children container;
        run =
          if container
          then ""
          else base.run;
        description =
          if container && base.description == ""
          then
            (
              if count == 1
              then "1 subcommand"
              else "${toString count} subcommands"
            )
          else base.description;
      };
  in
    lib.sort byLabel (map node (lib.unique (map word entries)));

  # Every node below a top-level command runs the way that command does:
  # bare words when it is on PATH, else through `x`.
  withPlacement = onPath: node:
    node
    // {
      inherit onPath;
      command =
        if onPath
        then node.name
        else node.xInvocation;
      children = map (withPlacement onPath) node.children;
    };

  # The top-level nodes of the catalogue tree.
  commandNodes = commands: let
    nodes = map (node: withPlacement (node.direct && !node.synthesized) node) (
      buildNodes 0 (normalizeCommandEntries commands)
    );
  in
    assert checkInvocations (flattenNodes nodes); nodes;

  # Every node, parents before their subcommands.
  flattenNodes = nodes: lib.concatMap (node: [node] ++ flattenNodes node.children) nodes;

  normalizeCommandGroups = groupOrder: commands: let
    nodes = commandNodes commands;
    availableGroups = lib.unique (map (node: node.group) nodes);
    # Commands without a group have no heading, so they list above every
    # group; placed lower, they would read as part of the group above them.
    ungrouped = lib.optional (lib.elem "" availableGroups) "";
    named = lib.filter (group: group != "") availableGroups;
    requestedGroups = ["prelude"] ++ groupOrder;
    preferredGroups = lib.unique (lib.filter (group: lib.elem group named) requestedGroups);
    remainingGroups = lib.sort builtins.lessThan (
      lib.filter (group: !lib.elem group preferredGroups) named
    );
    groupNames = ungrouped ++ preferredGroups ++ remainingGroups;
  in
    assert lib.assertMsg (
      lib.unique groupOrder == groupOrder
    ) "prelude: sort.groups must not contain duplicates";
      map (group: {
        title = group;
        tasks = lib.filter (node: node.group == group) nodes;
      })
      groupNames;

  # Top-level nodes in group order.
  flatCommands = groups: lib.concatMap (group: group.tasks) groups;

  # Select commands for the MOTD Getting Started list.
  # - Commands with `motd` set appear at that sort order, at any depth.
  # - `x` is always included when present (opens the command palette).
  # - Each row is the command's user-runnable form: bare words when its root
  #   is on PATH (`db migrate`), else the `x` dispatch form.
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
      inherit (entry) name command description;
    })
    sorted;

  # --- projections -------------------------------------------------------------

  orEmpty = v:
    if v == null
    then ""
    else v;

  # Menu TUI JSON boundary: groups of tasks with the fields Go menu.Config
  # expects. Keeps catalogue metadata (usage/details/examples/args/key) intact;
  # `children` appears only on nodes that have subcommands. `command` is the
  # user-runnable form; consumers such as the shell status host must use it
  # rather than reconstructing invocation rules from a display label.
  projectTask = t:
    {
      name = t.name;
      label = t.label;
      run = t.run;
      command = t.command;
      description = t.description;
      key = orEmpty t.key;
      usage = orEmpty t.usage;
      details = orEmpty t.details;
      examples = t.examples;
      args = t.args;
    }
    // lib.optionalAttrs (t.children != []) {
      children = map projectTask t.children;
    };

  projectMenuGroups = groupOrder: commands:
    map (group: {
      title = group.title;
      tasks = map projectTask group.tasks;
    }) (normalizeCommandGroups groupOrder commands);

  # Every node (subcommands included) used by motd.nix before row reduction.
  projectMotdCatalog = groupOrder: commands: flattenNodes (flatCommands (normalizeCommandGroups groupOrder commands));

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
    commandNodes
    flattenNodes
    normalizeCommandGroups
    flatCommands
    selectCommands
    projectMenuGroups
    projectMotdCatalog
    projectMotdRows
    ;
}
