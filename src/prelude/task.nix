{ lib }:
{
  package ? null,
  executable ? null,
  command ? null,
  program ? null,
  arguments ? [ ],
  ...
}@spec:
let
  sources = lib.count (value: value != null) [ package executable command ];
  selectedExecutable =
    if package == null then
      executable
    else if program == null then
      lib.getExe package
    else
      lib.getExe' package program;
  run =
    if command != null then
      command
    else
      lib.concatMapStringsSep " " lib.escapeShellArg ([ selectedExecutable ] ++ arguments);
in
assert lib.assertMsg (sources == 1)
  "prelude.lib.mkTask: set exactly one of `package`, `executable`, or `command`";
assert lib.assertMsg (program == null || package != null)
  "prelude.lib.mkTask: `program` requires `package`";
(removeAttrs spec [
  "package"
  "executable"
  "command"
  "program"
  "arguments"
])
// {
  inherit run;
  runtimePackages = lib.optional (package != null) package;
}
