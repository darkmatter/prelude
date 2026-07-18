# Adapt a package derivation to Prelude's command schema. The package determines
# the executable and runtime closure; callers provide only command metadata and
# optional `program` / `arguments` overrides.
{ lib }:
let
  mkCommand = import ./task.nix { inherit lib; };
in
package: extras:
assert lib.assertMsg (lib.isDerivation package)
  "prelude.lib.fromPkg: expected a package derivation";
assert lib.assertMsg (builtins.isAttrs extras)
  "prelude.lib.fromPkg: expected command extras as an attribute set";
mkCommand (extras // { inherit package; })
