# Per-system packages. Component and CLI packages come from the Prelude module;
# this adds repository-only generators, demos, and the default package used by
# fragmentless `nix run <flake> -- <command>`.
{
  config,
  demos,
  docsAutomation,
  previews,
  skill,
  ...
}:
{
  default = config.packages.prelude;
  inherit previews skill;
  docs-record = docsAutomation.record;
  docs-sync = docsAutomation.sync;
}
// demos.examplePackages
