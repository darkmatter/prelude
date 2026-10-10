# prelude.menu.* options — the interactive command menu (bubbletea TUI).
{lib, ...}: let
  defaults = import ../defaults.nix;
  t = import ../option-types.nix {inherit lib;};
in {
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

    builtins = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "List prelude's own navigation commands (x, docs, portal) in the menu catalogue. Set to false when an imported catalogue (e.g. Justfile recipes) should own the menu and the navigation entries read as noise.";
    };

    just = {
      enable = lib.mkEnableOption "importing public Justfile recipes into the command menu";

      justfile = lib.mkOption {
        type = lib.types.nullOr lib.types.path;
        default = null;
        description = "Optional Justfile path; null uses just's normal Justfile discovery at runtime, from the menu's root or, without one, from the caller's directory.";
      };

      group = lib.mkOption {
        type = lib.types.str;
        default = "just";
        description = "Menu group for imported ungrouped Justfile recipes.";
      };
    };

    scripts = {
      enable = lib.mkOption {
        type = lib.types.bool;
        default = defaults.menu.scripts.enable;
        example = true;
        description = "Import package.json scripts into the command menu at runtime. Each script runs exactly as written, from its package.json directory with node_modules/.bin ahead of PATH. No package manager runs it, so pre/post scripts and npm_* variables do not apply.";
      };

      packageJson = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = defaults.menu.scripts.packageJson;
        example = "web/package.json";
        description = "package.json to import. null uses the nearest package.json at or above the menu's root (or, without one, the caller's directory) at runtime. A relative path resolves from the nearest directory holding flake.nix. A string, not a Nix path, so it is never copied into the store away from its node_modules.";
      };

      group = lib.mkOption {
        type = lib.types.str;
        default = defaults.menu.scripts.group;
        description = "Menu group for imported package.json scripts whose names have no `:` or `/`.";
      };
    };
  };
}
