# The Go module's build inputs, shared by every Go builder so they all resolve
# to one source store path.
#
# An unfiltered `src = ../.` hashes the whole src/ tree, so any edit to a Nix
# generator, shell module, or font would recompile every binary — and nix-direnv
# waits for that before the shell loads. Only go.mod, go.sum, Go sources, and
# the templates the wizard embeds can change a binary. Tests are left out too:
# every builder sets doCheck = false, and `x go:test` runs them from the
# working tree.
{lib}: let
  fs = lib.fileset;
  isGoInput = file:
    (file.hasExt "go" && !lib.hasSuffix "_test.go" file.name)
    || file.hasExt "tmpl";
in
  fs.toSource {
    root = ../.;
    fileset = fs.unions [
      ../go.mod
      ../go.sum
      (fs.intersection (fs.fileFilter isGoInput ../.) (fs.unions [
        # A separate Go module with its own go.mod; the main module never
        # builds it.
        (fs.difference ../cmd ../cmd/prelude-shell-vt-host)
        ../internal
        ../pkg
      ]))
    ];
  }
