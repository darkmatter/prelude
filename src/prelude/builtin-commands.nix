# Prelude's own catalogue entries: menu, docs, portal and workspace launchers. A
# plain module, shared by flakeModules.default and lib.evalModule.
{
  lib,
  config,
  ...
}: let
  cfg = config.prelude;
  docsEnabled = cfg.docs.pages != [];
in {
  # Prelude owns its navigation commands and default accelerators. Consumers
  # can still override any field explicitly, while project command catalogues
  # stay focused on lifecycle actions such as serve, build, test, and install.
  config.prelude.commands = lib.mkMerge [
    (lib.mkIf cfg.menu.enable {
      x = {
        description = lib.mkDefault "open the interactive command menu";
        exec = lib.mkDefault "x";
        key = lib.mkDefault "m";
      };
    })
    (lib.mkIf docsEnabled {
      docs = {
        description = lib.mkDefault "browse project documentation";
        exec = lib.mkDefault "docs";
        key = lib.mkDefault "d";
      };
    })
    (lib.mkIf cfg.workspace.enable {
      "prelude:workspace" = {
        description = lib.mkDefault "open the Bash workspace with menu and docs panes";
        # Resolve from PATH: a package-backed entry would cycle through the
        # workspace's own menu package and its command catalogue.
        exec = lib.mkDefault "prelude-workspace";
        group = lib.mkDefault "prelude";
      };
    })
    (lib.mkIf cfg.portal.enable {
      portal = {
        description = lib.mkDefault "launch an app, with live health lights";
        exec = lib.mkDefault "portal";
        # `p` rather than a mnemonic for "web": the terminal front end is the
        # default, and `m`/`d` are already taken by the menu and docs.
        key = lib.mkDefault "p";
      };
    })
  ];
}
