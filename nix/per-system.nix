# Per-system outputs composition root. motd/menu (packages + apps) come
# from the prelude module itself; everything else layers on top:
#
#   demos.nix     shared feature-demo builders (evaluated once)
#     └ checks.nix     build + render smoke tests
#         └ previews.nix   utility that builds render checks and shows them
#             └ packages.nix / apps.nix / shell.nix
{
  pkgs,
  lib,
  config,
  ...
}:
let
  args = { inherit pkgs lib config; };
  demos = import ./demos.nix args;
  docsAutomation = import ./docs-automation.nix args;
  # Mutually recursive but well-founded: previews only reads the (static)
  # attribute names of checks, while one check value resolves advertised
  # motd commands against the previews package.
  checks = import ./checks.nix (args // { inherit demos docsAutomation previews; });
  previews = import ./previews.nix (args // { inherit checks; });
in
{
  packages = import ./packages.nix (args // { inherit demos docsAutomation previews; });
  apps = import ./apps.nix (args // { inherit demos docsAutomation previews; });
  devShells.default = import ./shell.nix (args // { inherit docsAutomation previews; });
  formatter = pkgs.nixfmt;
  inherit checks;
}
