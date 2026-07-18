# Showcase: configure Prelude's static welcome banner.
{ ... }:
{
  prelude.motd = {
    enable = true;
    title = {
      text = ./title.txt;
      align = "center";
      style = "spine";
    };
    maxWidth = 88;
    windowBackground = false;
    background = false;
    clearScreen = true;
    # Bottom breathing room only when the terminal can afford it; short
    # windows drop it instead of scrolling the card away.
    margin = {
      bottom = 6;
      minHeight = 40;
    };
    padding.x = 4;
    padding.bottom = 2;
    padding.top = 1;

    header = {
      tagline = {
        text = "Devshell UI for Nix flakes";
        subtitle = "MOTD, command menu, docs viewer, and prompt from one flake-parts module";
        layout = "stack";
        align = "left";
      };
      background = false;
      statusHint = {
        layout = "inline";
        links = [
          {
            label = "github";
            url = "https://github.com/darkmatter/prelude";
          }
        ];
      };
      status = {
        flake = {
          order = 100;
          label = "flake check";
          # Header probes should stay cheap: evaluate all checks, but leave
          # their builds to the explicit `check` menu command.
          check = "nix flake check --no-build >/dev/null 2>&1";
          output = "light";
        };
      };
    };

    description.text = ''
      You are inside Prelude's own devshell — the banner, menu, docs, and prompt around you are built by this repo from examples/default. Go renderers live in src/, the flake-parts modules in src/prelude/, and this configuration in examples/default/. Run `menu` to browse every command, or `docs` for the guides — including how to set up Prelude in your own repo.
    '';

    env = [ ];

    # Commands shown in Getting Started are selected via `commands.<name>.motd`
    # (sort order) — see menu.nix. Recipes are separate multi-step workflows.

    recipes.your-own-repo = {
      order = 100;
      title = "set up prelude in your own repo";
      steps = [
        { comment = "start from the copyable consumer flake"; }
        { command = "cp -r examples/reference ../my-shell && cd ../my-shell"; }
        { command = "nix develop   # input is github:darkmatter/prelude"; }
        { comment = "full walkthrough: docs, page \"Your own repo\""; }
      ];
    };

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

  };
}
