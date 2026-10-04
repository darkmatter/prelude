# Root-only apps layered on top of the module's single public `prelude` app.
# `examples` and `previews` are repository development surfaces, and `docs`
# shows the docs of whatever repository it runs in; importing the Prelude
# module adds none of them to a consumer's outputs or devshell.
{
  lib,
  demos,
  previews,
  ...
} @ args: let
  mkApp = pkg: {
    type = "app";
    program = lib.getExe pkg;
  };
in {
  docs = mkApp (import ./docs-app.nix args);
  examples = mkApp demos.examplesRunner;
  previews = mkApp previews;
}
