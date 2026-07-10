# Dogfood devshell: greeted by our own motd; `menu` drives the project
# from inside the shell.
{ pkgs, config, previews, ... }:
pkgs.mkShell {
  packages = [
    config.packages.motd
    config.packages.menu
    previews
  ] ++ (with pkgs; [
    gum
    shellcheck
    nixfmt
  ]);
  shellHook = ''
    motd
  '';
}
