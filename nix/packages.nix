# Per-system packages. motd/menu come from the prelude module
# (config.packages.*); this adds the default alias, the previews utility,
# and the feature demos.
{ config
, demos
, docsAutomation
, previews
, ...
}:
{
  default = config.packages.setup;
  inherit previews;
  docs-record = docsAutomation.record;
  docs-sync = docsAutomation.sync;
}
  // demos.examplePackages
