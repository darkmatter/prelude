# Showcase: themed starship prompt. `packages.prompt` is a starship.toml;
# the devshell exports STARSHIP_CONFIG=<it> (see nix/shell.nix) and the
# user's existing starship picks it up on the next prompt render.
{ ... }:
{
  prelude.prompt.enable = true;
}
