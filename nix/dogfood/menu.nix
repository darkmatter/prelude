# Showcase: configure the command menu and its command catalogue.
{ self, ... }:
{
  prelude = {
    menu.enable = true;
    prompt.enable = true;
    commands = {
      motd = {
        description = "reprint the welcome banner";
        group = "general";
        order = 100;
      };
      menu = {
        description = "open this command menu";
        group = "general";
        order = 200;
      };
      docs = {
        description = "open the project manual (sidebar + number keys)";
        group = "general";
        order = 300;
      };

      # `exec = <own name>` marks a command already provided by the shell.
      previews = {
        description = "build the render checks and show their output";
        exec = "previews";
        group = "develop";
        key = "p";
        order = 400;
      };
      titles = {
        description = "inspect rendered titles";
        exec = "prelude-title-previews prelude";
        group = "develop";
        order = 500;
      };
      docs-sync = {
        description = "regenerate option and showcase markdown";
        exec = "docs-sync";
        group = "develop";
        order = 600;
      };
      docs-record = {
        description = "record stale VHS showcases and sync docs";
        exec = "docs-record";
        group = "develop";
        order = 700;
      };
      build = {
        description = "build a flake output";
        exec = "nix build";
        group = "develop";
        key = "b";
        order = 800;
        usage = "menu build .#motd";
        args = [
          {
            token = "<target>";
            description = "flake output to build";
            options = [
              ".#motd"
              ".#menu"
              ".#example-themes"
            ];
          }
        ];
      };

      examples = {
        description = "tour every feature demo";
        exec = "nix run .#examples";
        group = "demos";
        key = "e";
        order = 900;
      };
      themes = {
        description = "render a mini motd per theme";
        exec = "nix run .#example-themes";
        group = "demos";
        key = "t";
        order = 1000;
      };
      demo-motd = {
        description = "acme-web welcome banner demo";
        exec = "nix run .#example-motd";
        group = "demos";
        order = 1100;
      };
      demo-menu = {
        description = "acme-web command menu demo (arg entry)";
        exec = "nix run .#example-menu";
        group = "demos";
        order = 1200;
      };
    };
  };

  # Package-backed commands derive both their executable and runtime closure.
  perSystem =
    { pkgs, ... }:
    {
      prelude.commands = {
        check = self.lib.mkCommand {
          command = "nix flake check";
          description = "build + render smoke tests";
          group = "develop";
          key = "c";
          order = 850;
        };
        fmt = self.lib.mkCommand {
          package = pkgs.nixfmt;
          arguments = [ "." ];
          description = "format nix sources";
          group = "develop";
          key = "f";
          order = 875;
        };
      };
    };
}
