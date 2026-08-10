# Per-system apps. motd/menu apps come from the prelude module
# (including the standalone agent skill), the demo runner, previews, and one
# app per feature demo.
{
  lib,
  config,
  demos,
  docsAutomation,
  previews,
  skill,
  ...
}: let
  mkApp = pkg: {
    type = "app";
    program = lib.getExe pkg;
  };
in
  {
    default = config.apps.menu;
    examples = mkApp demos.examplesRunner;
    previews = mkApp previews;
    docs-record = mkApp docsAutomation.record;
    docs-sync = mkApp docsAutomation.sync;
    skill = mkApp skill;
  }
  // lib.mapAttrs (_name: mkApp) demos.examplePackages
