# prelude.docs.* options — hand-authored man-style project manual.
{ lib, ... }:
let
  defaults = import ../defaults.nix;
  t = import ../option-types.nix { inherit lib; };

  # One content block. Only the fields relevant to `type` are used.
  blockType = lib.types.submodule {
    options = {
      type = lib.mkOption {
        type = lib.types.enum [
          "lead"
          "para"
          "paragraph"
          "option"
          "command"
          "shell"
          "example"
          "blank"
        ];
        description = ''
          Block kind:
          - lead: `term — text` (name line)
          - para: wrapped body paragraph
          - option / command: bold term + indented description
          - shell / example: `$ command` with optional note
          - blank: empty row
        '';
      };
      term = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Highlighted term (lead / option / command).";
      };
      text = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Body text (lead summary, paragraph, option/command description).";
      };
      command = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Shell command for shell/example blocks.";
      };
      note = lib.mkOption {
        type = lib.types.str;
        default = "";
        description = "Dim note under a shell/example line.";
      };
    };
  };

  sectionType = lib.types.submodule {
    options = {
      order = lib.mkOption {
        type = lib.types.int;
        default = 1000;
        description = "Sidebar order; section key breaks ties.";
      };
      title = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Sidebar label; defaults to the section attribute name.";
      };
      blocks = lib.mkOption {
        type = lib.types.listOf blockType;
        default = [ ];
        description = "Hand-authored body blocks for this section. Nothing is auto-generated.";
      };
    };
  };
in
{
  options.prelude.docs = {
    enable = lib.mkEnableOption "project docs manual (man-style TUI)";

    sections = lib.mkOption {
      type = lib.types.attrsOf sectionType;
      default = defaults.docs.sections;
      description = ''
        Hand-authored manual sections, keyed by identity. Each section becomes a
        CONTENTS sidebar entry; digits 1–9 jump to them. Write the body yourself
        with `blocks` — the viewer does not invent content from menu groups.
      '';
      example = {
        name = {
          order = 100;
          blocks = [
            {
              type = "lead";
              term = "acme";
              text = "a tiny project runner";
            }
          ];
        };
        synopsis = {
          order = 200;
          blocks = [
            {
              type = "shell";
              command = "menu";
            }
          ];
        };
      };
    };
  };
}
