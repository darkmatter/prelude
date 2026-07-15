# Showcase: configure Prelude's static welcome banner.
{ ... }:
{
  prelude.motd = {
    enable = true;
    title = ../title.txt;

    maxWidth = 80;
    windowBackground = { relative = 0.05; };
    background = null;
    clearScreen = true;
    margin.bottom = 10;
    padding.x = 4;
    padding.bottom = 2;
    padding.top = 1;

    header = {
      titleStyle = "spine";
      tagline = "Dev Shell Activated";
      subtitle = "Your environment is ready";
      taglineLayout = "stack";
      background = null;
      status = {
        nix = {
          order = 100;
          label = "nix";
          check = "nix --version >/dev/null 2>&1";
          output = "light";
        };
        flake = {
          order = 200;
          label = "flake";
          check = "nix flake check >/dev/null 2>&1";
          output = "light";
        };
      };
    };

    description.text = ''
      This shell pins every tool the repo needs — compilers, linters, and language servers are already on your PATH. No global installs, and your host machine stays untouched.
    '';

    env = [ ];

    # Menu task names: descriptions and wrappers come from the shared catalogue.
    commands = [
      "menu"
      "check"
      "previews"
      "titles"
    ];

    recipes.tour-demos = {
      order = 200;
      title = "tour the feature demos";
      steps = [
        { comment = "acme-web showcase, then every theme"; }
        { command = "nix run .#example-motd"; }
        { command = "nix run .#example-themes"; }
        { command = "nix run .#examples"; }
      ];
    };

    shortcuts = [
      {
        command = "help";
        alias = "?";
      }
      {
        command = "menu";
        alias = "m";
      }
      {
        command = "docs";
        alias = "d";
      }
    ];
  };
}
