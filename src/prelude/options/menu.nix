# prelude.menu.* options — the interactive command menu (bubbletea TUI).
{ lib, ... }:
let
  defaults = import ../defaults.nix;
  t = import ../option-types.nix { inherit lib; };
in
{
  options.prelude.menu = {
    enable = lib.mkEnableOption "interactive devshell command menu";

    placeholder = lib.mkOption {
      type = lib.types.str;
      default = defaults.menu.placeholder;
      description = "Placeholder text in the filter input.";
    };

    height = lib.mkOption {
      type = lib.types.ints.positive;
      default = defaults.menu.height;
      description = "Filter list height in rows.";
    };

    execute = lib.mkOption {
      type = lib.types.bool;
      default = defaults.menu.execute;
      description = "Execute the selected command (exec bash -c). When false, print it instead.";
    };

    width = lib.mkOption {
      type = t.widthType;
      default = defaults.menu.width;
      description = "Menu width, or \"full\" to fill the terminal width.";
    };

    maxWidth = lib.mkOption {
      type = lib.types.nullOr lib.types.ints.unsigned;
      default = defaults.menu.maxWidth;
      description = "Maximum menu width.";
    };
  };
}
