# Runnable feature demos for the prelude components.
#
# Motd demos become packages/apps named `example-<name>`:
#
#   nix run .#example-minimal
#
# The acme-web motd/menu demos back `nix run .#example-motd` and
# `nix run .#example-menu`, and `nix run .#example-themes` renders a mini
# motd per theme. Render everything:
#
#   nix run .#examples
#
# Demos disable clearScreen and the top margin so they can render one after
# another.
let
  # Demo task groups, ported from the design's lib/devshell.ts + commands.ts.
  # Exercises: keys, run-vs-name, details/usage/examples, optional args with
  # suggestions, boolean flags, required positional args, and free text.
  groups = {
    general = {
      order = 100;
      tasks = {
        menu = {
          order = 100;
          description = "open the interactive command menu";
        };
        clean = {
          order = 200;
          run = "rm -rf .next .turbo node_modules/.cache";
          description = "remove build artifacts & caches";
        };
      };
    };
    develop = {
      order = 200;
      tasks = {
        dev = {
          order = 100;
          run = "pnpm dev";
          description = "start the dev server with hot reload";
          key = "d";
          usage = "menu dev --port 3000";
          details = "Boots a development server that watches the source tree and hot-reloads modules as files change. Binds to 127.0.0.1:3000 by default; override with --port and --host.";
          examples = [
            "menu dev --port 8080"
            "menu dev --host 0.0.0.0"
          ];
          args = [
            {
              token = "--port";
              description = "Port to bind the dev server";
              options = [
                "3000"
                "8080"
              ];
            }
            {
              token = "--host";
              description = "Interface to expose";
              options = [
                "127.0.0.1"
                "0.0.0.0"
              ];
            }
          ];
        };
        build = {
          order = 200;
          run = "pnpm build";
          description = "compile an optimized production bundle";
          key = "b";
        };
        test = {
          order = 300;
          run = "pnpm test";
          description = "run the unit test suite";
          key = "t";
        };
      };
    };
    database = {
      order = 300;
      tasks = {
        "db:up" = {
          order = 100;
          run = "docker compose up -d db redis";
          description = "start postgres & redis in the background";
        };
        "db:migrate" = {
          order = 200;
          run = "drizzle-kit migrate";
          description = "apply pending schema migrations";
          key = "m";
        };
      };
    };
    ops = {
      order = 400;
      tasks = {
        deploy = {
          order = 100;
          run = "vercel deploy";
          description = "ship the current build to production";
          usage = "menu deploy --alias staging";
          details = "Uploads the most recent production build and promotes it to the live environment. Deploys are atomic: traffic switches only after the new release passes its health checks.";
          examples = [
            "menu deploy --dry-run"
            "menu deploy --alias staging"
          ];
          args = [
            {
              token = "--alias";
              description = "Publish to a named preview URL";
              options = [
                "staging"
                "preview"
              ];
            }
            {
              token = "--dry-run";
              description = "Print the manifest without shipping";
              boolean = true;
            }
          ];
        };
        push = {
          order = 200;
          run = "git push";
          description = "publish the current branch to the remote";
          key = "p";
          args = [
            {
              token = "<remote>";
              description = "Remote to push to";
              required = true;
              options = [
                "origin"
                "upstream"
              ];
            }
            {
              token = "<branch>";
              description = "Branch to publish";
              options = [
                "main"
                "dev"
              ];
            }
          ];
        };
      };
    };
  };

  # `nix run .#example-motd` — the full acme-web welcome banner: status
  # chips in the banner, description, env chips, next steps, and recipes.
  motd = {
    project = "acme-web";
    banner.tagline = "everything you need to build, test & ship";
    clearScreen = false;
    margin.top = 0;
    banner.statusItems = [
      {
        text = "devshell";
        status = "success";
      }
      {
        text = "services";
        status = "warning";
      }
    ];
    description.text = "This repo uses nix-based tooling which provides a consistent and reproducible dev environment.";
    env = [
      {
        label = "node";
        value = "22.3.0";
      }
      {
        label = "pnpm";
        value = "9.4.0";
      }
      {
        label = "postgres";
        value = "16.3";
      }
    ];
    commands = {
      menu = {
        order = 100;
        command = "menu";
        description = "browse all project commands";
      };
      dev = {
        command = "just dev";
        description = "start local development";
      };
      test = {
        command = "just test";
        description = "run the test suite";
      };
    };
    recipes = {
      clean-local-stack = {
        title = "spin up a clean local stack";
        lines = [
          "# Start backing services"
          "just db:up"
          "just db:migrate && just db:seed"
          "just dev"
        ];
      };
      ship-hotfix = {
        title = "ship a hotfix to production";
        lines = [
          "git checkout -b fix/login"
          ""
          "# Verify before deploying"
          "just test && just build"
          "just deploy"
        ];
      };
    };
    git = true;
  };

  # `nix run .#example-menu` — the interactive command menu (`menu list`
  # for CI).
  menu = {
    project = "acme-web";
    inherit groups;
  };

  # --- motd feature demos --------------------------------------------------------

  motdDemos = {
    # The merged component doing the former card's job: a standalone
    # banner + description, no commands, env, footer, or load line.
    minimal = {
      project = "minimal";
      banner.tagline = "just a banner and a description";
      clearScreen = false;
      margin.top = 0;
      align = "left";
      loadLine = "";
      footer = false;
      git = false;
      description = {
        text = "Explicit styling beats the theme — this line is italic with a custom color.";
        foreground = "#8be9fd";
        italic = true;
      };
    };

    # Window background fill (theme bg token, edge to edge) + status chips
    # + thick border.
    surface = {
      project = "surface";
      banner.tagline = "windowBackground = true paints the whole window";
      clearScreen = false;
      margin.top = 1;
      margin.bottom = 1;
      windowBackground = true;
      banner.border.width = 2;
      loadLine = "";
      git = false;
      banner.statusItems = [
        {
          text = "api";
          status = "success";
        }
        {
          text = "db";
          status = "error";
        }
        {
          text = "cache";
          status = "warning";
        }
        {
          text = "docs";
          status = "info";
        }
      ];
      description.text = "Every cell, gutter, and line remainder carries the background.";
      footer = false;
    };
  };

  # `nix run .#example-themes` — a mini motd per theme, background-filled
  # so the palettes read as intended.
  themeMotd = theme: {
    inherit theme;
    project = theme;
    banner.tagline = "theme ${theme}";
    clearScreen = false;
    margin.y = 2;
    maxWidth = 60;
    windowBackground = true;
    loadLine = "";
    git = false;
    banner.statusItems = [
      {
        text = "ok";
        status = "success";
      }
      {
        text = "warn";
        status = "warning";
      }
      {
        text = "err";
        status = "error";
      }
    ];
    description.text = "The quick brown fox jumps over the lazy dog.";
    commands.build = {
      command = "just build";
      description = "accent on commands";
    };
    footer = false;
  };
in
{
  inherit
    groups
    motd
    menu
    motdDemos
    themeMotd
    ;
}
