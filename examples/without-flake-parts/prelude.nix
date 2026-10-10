# The same shape the wizard writes and flakeModules.default imports. Under
# lib.evalModule the module receives `pkgs`, so a package-backed command needs
# no `perSystem`.
{
  lib,
  pkgs,
  ...
}: {
  prelude = {
    project = "without-flake-parts";
    motd.enable = true;
    menu.enable = true;
    # Opt in to the workspace; docs.pages below supplies its documentation.
    # workspace.enable = true;

    commands.hello = {
      description = "greet from a package";
      exec = lib.getExe pkgs.hello;
    };

    docs.pages = [
      {text = ./README.md;}
    ];
  };
}
