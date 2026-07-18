{ lib }:
{ package ? null
, executable ? null
, command ? null
, program ? null
, arguments ? [ ]
, invocation ? null
, ...
}@spec:
let
  sources = lib.count (value: value != null) [
    package
    executable
    command
  ];
  selectedExecutable =
    if package == null then
      executable
    else if program == null then
      lib.getExe package
    else
      lib.getExe' package program;
  exec =
    if command != null then
      command
    else
      lib.concatMapStringsSep " " lib.escapeShellArg ([ selectedExecutable ] ++ arguments);
  canonicalInvocation =
    if invocation != null then
      invocation
    else if command != null then
      command
    else
      lib.concatMapStringsSep " " lib.escapeShellArg ([ (baseNameOf selectedExecutable) ] ++ arguments);
in
assert lib.assertMsg
  (
    sources == 1
  ) "prelude.lib.mkCommand: set exactly one of `package`, `executable`, or `command`";
assert lib.assertMsg
  (
    program == null || package != null
  ) "prelude.lib.mkCommand: `program` requires `package`";
(removeAttrs spec [
  "package"
  "executable"
  "command"
  "program"
  "arguments"
  "invocation"
])
  // {
  inherit exec;
  invocation = canonicalInvocation;
  runtimePackages = lib.optional (package != null) package;
}
