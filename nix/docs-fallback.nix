# The docs of a repository with no Prelude configuration, for the `docs` app
# (docs-app.nix): Prelude's defaults over its README.md and docs/**/*.md,
# evaluated through lib.evalModule like any configured project, so there is
# no second way to describe docs.
{
  # Absolute path of the repository, as a string.
  root,
  # Prelude's own source.
  prelude,
  pkgs,
}: let
  inherit (pkgs) lib;
  preludeLib = import (prelude + "/nix/lib.nix") {inherit lib;};

  # Copy only the Markdown the viewer shows, never the rest of the tree
  # (node_modules, build output).
  source = builtins.path {
    path = root;
    name = "docs-source";
    filter = path: type: let
      relative = lib.removePrefix "${toString root}/" (toString path);
      underDocs = relative == "docs" || lib.hasPrefix "docs/" relative;
    in
      if type == "directory"
      then underDocs
      else relative == "README.md" || (underDocs && lib.hasSuffix ".md" relative);
  };
  readme = source + "/README.md";
  hasReadme = builtins.pathExists readme;
  docsFiles =
    lib.optionals (builtins.pathExists (source + "/docs"))
    (lib.filter (file: lib.hasSuffix ".md" (toString file)) (lib.filesystem.listFilesRecursive (source + "/docs")));

  evaluated = preludeLib.evalModule pkgs {
    prelude = {
      project = baseNameOf root;
      docs = {
        rootReadme =
          if hasReadme
          then readme
          else null;
        pages = lib.optional hasReadme (preludeLib.mdSplit readme) ++ map (text: {inherit text;}) docsFiles;
      };
    };
  };
in
  evaluated.packages.prelude-docs
