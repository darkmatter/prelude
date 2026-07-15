# Showcase: configure the command menu and its task catalogue.
{ self, ... }:
{
  prelude = {
    menu.enable = true;

    groups = {
      general = {
        order = 100;
        tasks = {
          motd = {
            order = 100;
            description = "reprint the welcome banner";
          };
          menu.description = "open this command menu";
          docs = {
            order = 300;
            description = "open the project manual (sidebar + number keys)";
          };
        };
      };

      develop.tasks = {
        # `run = <own name>` marks a command already provided by the shell.
        previews = {
          run = "previews";
          description = "build the render checks and show their output";
          key = "p";
        };
        titles = {
          run = "title-previews";
          description = "inspect rendered titles";
        };
        docs-sync = {
          run = "docs-sync";
          description = "regenerate option and showcase markdown";
        };
        docs-record = {
          run = "docs-record";
          description = "record stale VHS showcases and sync docs";
        };
        build = {
          run = "nix build";
          description = "build a flake output";
          key = "b";
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
      };

      demos.tasks = {
        examples = {
          run = "nix run .#examples";
          description = "tour every feature demo";
          key = "e";
        };
        themes = {
          run = "nix run .#example-themes";
          description = "render a mini motd per theme";
          key = "t";
        };
        demo-motd = {
          run = "nix run .#example-motd";
          description = "acme-web welcome banner demo";
        };
        demo-menu = {
          run = "nix run .#example-menu";
          description = "acme-web command menu demo (arg entry)";
        };
      };
    };
  };

  # Package-backed tasks derive both their command and runtime closure.
  perSystem =
    { pkgs, ... }:
    {
      prelude.groups.develop.tasks = {
        check = self.lib.mkTask {
          command = "nix flake check";
          description = "build + render smoke tests";
          key = "c";
        };
        fmt = self.lib.mkTask {
          package = pkgs.nixfmt;
          arguments = [ "." ];
          description = "format nix sources";
          key = "f";
        };
      };
    };
}
