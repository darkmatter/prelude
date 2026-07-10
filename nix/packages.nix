# Per-system packages. motd/menu come from the prelude module
# (config.packages.*); this adds the default alias, the previews utility,
# and the feature demos.
{ config, demos, previews, ... }:
{
  default = config.packages.motd;
  inherit previews;
}
// demos.examplePackages
