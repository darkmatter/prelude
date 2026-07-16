# prelude.docs.* options — Markdown pages rendered in a navigable TUI.
{ lib, ... }:
let
  defaults = import ../defaults.nix;

  pageType = lib.types.submodule {
    options.text = lib.mkOption {
      type = lib.types.path;
      description = "Path to a Markdown file. Its first level-one heading labels the sidebar entry.";
    };
  };
in
{
  options.prelude.docs.pages = lib.mkOption {
    type = lib.types.listOf pageType;
    default = defaults.docs.pages;
    description = ''
      Markdown pages shown in declaration order. Each file becomes one CONTENTS
      sidebar entry; digits 1–9 jump between pages. The first level-one heading
      labels the entry and the complete file is rendered as Markdown.
    '';
    example = lib.literalExpression ''
      [
        { text = ./docs/getting-started.md; }
        { text = ./docs/commands.md; }
      ]
    '';
  };
}
