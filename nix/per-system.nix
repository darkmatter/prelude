# Per-system outputs composition root. motd/menu (packages + apps) come
# from the prelude module itself; everything else layers on top:
#
#   demos.nix     shared feature-demo builders (evaluated once)
#     └ checks.nix     build + render smoke tests
#         └ previews.nix   utility that builds render checks and shows them
#             └ packages.nix / apps.nix / shell.nix
{ pkgs, lib, config, ... }:
let
  args = { inherit pkgs lib config; };
  demos = import ./demos.nix args;
  checks = import ./checks.nix (args // { inherit demos; });
  previews = import ./previews.nix (args // { inherit checks; });
in
{
  packages = import ./packages.nix (args // { inherit demos previews; });
  apps = import ./apps.nix (args // { inherit demos previews; });
  devShells.default = import ./shell.nix (args // { inherit previews; });
  formatter = pkgs.nixfmt;
  inherit checks;
}
