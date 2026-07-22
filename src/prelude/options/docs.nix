# prelude.docs.* — nested Markdown nav + optional nixosOptionsDoc source.
{ lib, ... }:
let
  defaults = import ../defaults.nix;
  inherit (lib) mkOption types;

  # True recursive nav node. children is visible = "shallow" so nixosOptionsDoc
  # documents the field once and does not descend into infinite pages.*.children.*
  # repetition (optionAttrSetToDocList treats "shallow" as visible but skips
  # sub-options). Do not use oneOf/fixType: submodule checks accept any attrset.
  nodeType = types.submodule {
    options = {
      text = mkOption {
        type = types.nullOr types.path;
        default = null;
        description = "Markdown file for a leaf node. First H1 labels the sidebar when title is null.";
      };
      title = mkOption {
        type = types.nullOr types.str;
        default = null;
        description = "Sidebar label for groups and generate nodes; optional override for leaves.";
      };
      children = mkOption {
        type = types.listOf nodeType;
        default = [ ];
        # Document this option, but do not walk nested children.* for the manual.
        visible = "shallow";
        description = "Child nodes for a group. Non-empty implies this node is a group.";
      };
      generate = mkOption {
        type = types.nullOr (types.enum [ "nixosOptions" ]);
        default = null;
        description = ''
          When set to "nixosOptions", expand using prelude.docs.nixosOptions
          (a pkgs.nixosOptionsDoc argument set). The node does not carry option records.
        '';
      };
      split = mkOption {
        type = types.enum [
          "allLeaves"
          "shallow"
        ];
        default = "allLeaves";
        description = ''
          Only for generate = "nixosOptions".
          "allLeaves" (default): nested sidebar tree of every terminal option,
          preserving full paths (prelude.motd.env.*.label, …).
          "shallow": one leaf — full nixosOptionsDoc pass-through. Want a
          different partition? Pass a narrower options attrset or multiple
          generate nodes.
        '';
      };
    };
  };
in
{
  options.prelude.docs = {
    pages = mkOption {
      type = types.listOf nodeType;
      default = defaults.docs.pages;
      description = ''
        Documentation nav tree shown in declaration order. Each node is a leaf
        Markdown page, a titled group of children, or a generate selector that
        expands prelude.docs.nixosOptions via pkgs.nixosOptionsDoc.
      '';
      example = lib.literalExpression ''
        [
          { text = ./docs/getting-started.md; }
          {
            title = "Guides";
            children = [
              { text = ./docs/guides/a.md; }
            ];
          }
          { generate = "nixosOptions"; title = "Options"; }
        ]
      '';
    };

    rootReadme = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = ''
        Path to the consumer's root README.md. When a pages leaf's `text` is
        exactly this path, the docs TUI treats it as the project README (hero
        title from prelude.project). Match is by path equality in Nix before
        pages are renamed into the docs bundle — never by basename inference.
      '';
      example = lib.literalExpression "self + /README.md";
    };

    # Full pkgs.nixosOptionsDoc argument set — pass-through, not a curated subset.
    # freeformType accepts every nixosOptionsDoc parameter (transformOptions,
    # documentType, variablelistId, optionIdPrefix, revision, baseOptionsJSON,
    # warningsAreErrors, …) while still typing the required `options` field.
    nixosOptions = mkOption {
      type = types.submodule {
        freeformType = types.attrsOf types.raw;
        options = {
          options = mkOption {
            type = types.lazyAttrsOf types.raw;
            default = { };
            description = "Module options attrset; same as nixosOptionsDoc's options argument.";
          };
        };
      };
      default = {
        options = { };
      };
      defaultText = lib.literalExpression "{ options = { }; }";
      description = ''
        Arguments passed through to pkgs.nixosOptionsDoc when a pages node has
        generate = "nixosOptions". Same shape as the public tutorial:

          prelude.docs.nixosOptions = { inherit (eval) options; };

        or with any other nixosOptionsDoc parameter:

          prelude.docs.nixosOptions = {
            inherit (eval) options;
            transformOptions = o: o // { declarations = [ ]; };
            documentType = "none";
            warningsAreErrors = false;
          };

        Never JSON-serialize this value; the docs package builder feeds it to
        nixosOptionsDoc and only embeds Markdown store paths in the bundle.
      '';
      example = lib.literalExpression ''
        { inherit (eval) options; }
      '';
    };
  };
}
