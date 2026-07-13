# This repo's own devshell UI, built by its own module (see flake.nix):
# `nix develop` greets with our motd and `menu` drives the project.
#
# MOTD composition mirrors src/motd-playground (card width, phosphor fill,
# header bar, tips, Getting Started, shortcuts) with prelude-real content.
{
  theme = "prelude";
  # Hook contexts (direnv, wrapped shells) sometimes capture stderr, which
  # is where gum sniffs the terminal — degrading truecolor to 256. Force
  # 24-bit so the shellHook motd matches an interactive `motd`.
  colorProfile = "truecolor";
  project = "prelude";

  groups = {
    general = {
      order = 100;
      tasks = {
        motd = {
          order = 100;
          description = "reprint the welcome banner";
        };
        menu = {
          description = "open this command menu";
        };
      };
    };
    develop = {
      tasks = {
        check = {
          run = "nix flake check";
          description = "build + render smoke tests";
          key = "c";
        };
        fmt = {
          run = "nix fmt";
          description = "format nix sources";
          key = "f";
        };
        previews = {
          run = "nix run .#previews";
          description = "build the render checks and show their output";
          key = "p";
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
    };
    demos = {
      tasks = {
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

  motd = {
    enable = true;

    # Playground geometry: fixed ~60-col card, solid theme fill, centered.
    maxWidth = 60;
    background = true;
    windowBackground = true;
    clearScreen = true;
    margin.bottom = 10;

    header = {
      titleStyle = "spine";
      tagline = "everything you need to build, test & ship";
      statusLabel = "nix develop  ·  flake";
      statusLabelCompact = "flake";
      statusText = "ready";
    };

    description = {
      text = "This shell pins every tool the repo needs — compilers, linters, and language servers are already on your PATH. No global installs, and your host machine stays untouched.";
      tips = [
        "first time here? run `menu` to browse project commands,"
        "then `nix flake check` to verify. Config lives in `flake.nix`."
      ];
    };

    env = [
      {
        label = "nix";
        # awk NF: robust against "nix (Nix) 2.x" and "nix (Determinate Nix 3.x) 2.x"
        probe = "nix --version | awk '{print $NF}'";
      }
      {
        label = "go";
        probe = "go env GOVERSION";
      }
    ];

    commands = {
      browse = {
        order = 100;
        command = "menu";
        description = "browse all project commands";
      };
      check = {
        order = 200;
        command = "nix flake check";
        description = "verify the flake";
      };
      previews = {
        order = 300;
        command = "nix run .#previews";
        description = "inspect rendered examples";
      };
    };

    recipes = {
      verify-before-push = {
        order = 100;
        title = "verify before you push";
        steps = [
          { comment = "format, check, then inspect renders"; }
          { command = "nix fmt"; }
          { command = "nix flake check"; }
          { command = "nix run .#previews"; }
        ];
      };
      tour-demos = {
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

  menu.enable = true;
}
