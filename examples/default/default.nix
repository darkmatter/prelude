# Showcase usage: this repository consumes Prelude exactly like a downstream
# flake-parts project. Each UI component is configured independently so readers
# can copy only the MOTD, menu, or docs setup they need.
{ ... }:
{
  imports = [
    ./motd.nix
    ./menu.nix
    ./docs.nix
    ./prompt.nix
  ];

  prelude = {
    theme = "minted";
    colorProfile = "truecolor";
    project = "prelude";
  };
}
