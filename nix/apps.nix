# Root-only apps layered on top of the module's single public `prelude` app.
# `examples` and `previews` are repository development surfaces; importing the
# Prelude module does not add either one to a consumer's outputs or devshell.
{
  lib,
  demos,
  previews,
  ...
}: let
  mkApp = pkg: {
    type = "app";
    program = lib.getExe pkg;
  };
in {
  examples = mkApp demos.examplesRunner;
  previews = mkApp previews;
}
