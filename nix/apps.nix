# Per-system apps. motd/menu apps come from the prelude module
# (config.apps.*); this adds the default alias, the demo runner, previews,
# and one app per feature demo.
{ lib
, config
, demos
, docsAutomation
, previews
, ...
}:
let
  mkApp = pkg: {
    type = "app";
    program = lib.getExe pkg;
  };
in
{
  default = config.apps.motd;
  examples = mkApp demos.examplesRunner;
  previews = mkApp previews;
  docs-record = mkApp docsAutomation.record;
  docs-sync = mkApp docsAutomation.sync;
}
  // lib.mapAttrs (_name: mkApp) demos.examplePackages
