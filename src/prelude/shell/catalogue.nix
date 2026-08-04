# Serialize Prelude's evaluated command model as Bash 3.2-compatible arrays.
# Runtime modules consume this data; they do not rediscover commands from PATH.
{ lib }:

{
  commandEntries ? [ ],
}:

let
  oneLine =
    value: lib.replaceStrings [ "\n" "\r" "\t" ] [ " " " " " " ] (if value == null then "" else value);
  visibleEntries = lib.filter (
    entry:
    !lib.elem entry.name [
      "menu"
      "docs"
    ]
  ) commandEntries;
  indexedEntries = lib.imap0 (index: entry: { inherit index entry; }) visibleEntries;
  directEntries = lib.filter (item: !item.entry.grouped) indexedEntries;
  arguments = lib.concatLists (
    lib.imap0 (
      commandIndex: entry:
      lib.imap0 (argumentIndex: argument: {
        inherit commandIndex argumentIndex;
        token = argument.token;
        description = oneLine argument.description;
        options = argument.options;
      }) entry.args
    ) visibleEntries
  );
  candidates = lib.concatMap (
    argument:
    map (candidate: argument // { value = candidate; }) (
      lib.unique (lib.optional (lib.hasPrefix "-" argument.token) argument.token ++ argument.options)
    )
  ) arguments;

  array = name: values: ''
    ${name}=(
    ${lib.concatMapStringsSep "\n" (value: "  ${lib.escapeShellArg (toString value)}") values}
    )
  '';
in
''
  # Generated from the same normalized command entries used by menu.
  # shellcheck shell=bash
  ${array "_prelude_catalogue_names" (map (entry: entry.name) visibleEntries)}
  ${array "_prelude_catalogue_descriptions" (map (entry: oneLine entry.description) visibleEntries)}
  ${array "_prelude_catalogue_candidate_commands" (
    map (candidate: candidate.commandIndex) candidates
  )}
  ${array "_prelude_catalogue_candidate_positions" (
    map (candidate: candidate.argumentIndex) candidates
  )}
  ${array "_prelude_catalogue_candidate_values" (map (candidate: candidate.value) candidates)}
  ${array "_prelude_catalogue_candidate_descriptions" (
    map (candidate: candidate.description) candidates
  )}

  ${array "_prelude_catalogue_direct_names" (map (item: item.entry.name) directEntries)}
  ${array "_prelude_catalogue_direct_indexes" (map (item: item.index) directEntries)}
''
