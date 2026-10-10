# Per-system packages. Component and CLI packages come from the Prelude module;
# this adds repository-only generators, demos, the TypeScript API's library,
# and the default package used by fragmentless `nix run <flake> -- <command>`.
{
  config,
  demos,
  docsAutomation,
  previews,
  skill,
  typescript,
  ...
}:
{
  default = config.packages.prelude;
  inherit previews skill;
  docs-record = docsAutomation.record;
  docs-sync = docsAutomation.sync;
  inherit (typescript) libprelude;
  ts-sync = typescript.sync;
}
// demos.examplePackages
