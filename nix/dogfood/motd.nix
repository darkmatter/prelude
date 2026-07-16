# Showcase: configure Prelude's static welcome banner.
{ ... }:
{
  prelude.motd = {
    enable = true;
    title = {
      text = ../title.txt;
      align = "center";
      style = "spine";
    };
    maxWidth = 88;
    windowBackground = false;
    background = false;
    clearScreen = true;
    margin.bottom = 10;
    padding.x = 4;
    padding.bottom = 2;
    padding.top = 1;

    header = {
      tagline = {
        text = "Dev Shell Activated";
        subtitle = "Your environment is ready";
        layout = "stack";
        align = "left";
      };
      background = false;
      statusHint.layout = "inline";
      status = {
        dev = {
          order = 100;
          label = "dev server";
          check = "bash -c ': </dev/tcp/127.0.0.1/\${PORT:-3000}' >/dev/null 2>&1";
          output = "light";
        };
        flake = {
          order = 200;
          label = "flake";
          # Header probes should stay cheap: evaluate all checks, but leave
          # their builds to the explicit `check` menu command.
          check = "nix flake check --no-build >/dev/null 2>&1";
          output = "light";
        };
      };
    };

    description.text = ''
      This shell pins every tool the repo needs — compilers, linters, and language servers are already on your PATH. No global installs, and your host machine stays untouched.
    '';

    env = [ ];

    # Command names: descriptions and wrappers come from the shared catalogue.
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
