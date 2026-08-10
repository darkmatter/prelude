# Per-system packages. motd/menu come from the prelude module
# (config.packages.*); this adds the default alias, the previews utility,
# and the standalone agent skill.
{
  config,
  demos,
  docsAutomation,
  previews,
  skill,
  ...
}:
{
  default = config.packages.setup;
  inherit previews skill;
  docs-record = docsAutomation.record;
  docs-sync = docsAutomation.sync;
}
// demos.examplePackages
