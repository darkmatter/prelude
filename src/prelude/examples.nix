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
  # Demo commands, ported from the design's lib/devshell.ts + commands.ts.
  # Exercises: keys, exec-vs-name, details/usage/examples, optional args with
  # suggestions, boolean flags, required positional args, and free text.
  commands = {
    menu = {
      description = "open the interactive command menu";
      group = "general";
      order = 100;
    };
    clean = {
      description = "remove build artifacts & caches";
      exec = "rm -rf .next .turbo node_modules/.cache";
      group = "general";
      order = 200;
    };
    dev = {
      description = "start the dev server with hot reload";
      exec = "pnpm dev";
      group = "develop";
      key = "d";
      order = 300;
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
      description = "compile an optimized production bundle";
      exec = "pnpm build";
      group = "develop";
      key = "b";
      order = 400;
    };
    test = {
      description = "run the unit test suite";
      exec = "pnpm test";
      group = "develop";
      key = "t";
      order = 500;
    };
    "db:up" = {
      description = "start postgres & redis in the background";
      exec = "docker compose up -d db redis";
      group = "database";
      order = 600;
    };
    "db:migrate" = {
      description = "apply pending schema migrations";
      exec = "drizzle-kit migrate";
      group = "database";
      key = "m";
      order = 700;
    };
    deploy = {
      description = "ship the current build to production";
      exec = "vercel deploy";
      group = "ops";
      order = 800;
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
      description = "publish the current branch to the remote";
      exec = "git push";
      group = "ops";
      key = "p";
      order = 900;
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

  # `nix run .#example-motd` — full acme-web welcome banner.
  motd = {
    project = "acme-web";
    commandCatalog = commands;
    header = {
      tagline.text = "everything you need to build, test & ship";
      status.ready = {
        label = "devshell";
        status = "ready";
      };
    };
    clearScreen = false;
    margin.top = 0;
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
    commands = [
      "menu"
      "dev"
      "test"
    ];
    recipes = {
      clean-local-stack = {
        title = "spin up a clean local stack";
        steps = [
          { comment = "start postgres + redis first"; }
          { command = "just db:up"; }
          { command = "just db:migrate && just db:seed"; }
          { command = "just dev"; }
        ];
      };
      ship-hotfix = {
        title = "ship a hotfix to production";
        steps = [
          { command = "git checkout -b fix/login"; }
          { comment = "verify before deploying"; }
          { command = "just test && just build"; }
          { command = "just deploy"; }
        ];
      };
    };
    shortcuts = [
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

  # `nix run .#example-menu` — the interactive command menu (`menu list`
  # for CI).
  menu = {
    project = "acme-web";
    inherit commands;
  };

  # --- motd feature demos --------------------------------------------------------

  motdDemos = {
    # Standalone header + description, no commands/env/shortcuts.
    minimal = {
      project = "minimal";
      header.tagline.text = "just a header and a description";
      clearScreen = false;
      margin.top = 0;
      align = "left";
      description = {
        text = "Explicit styling beats the theme — this line is italic with a custom color.";
        foreground = "#8be9fd";
        italic = true;
      };
    };

    # Window background fill + status on the header bar.
    surface = {
      project = "surface";
      header = {
        tagline.text = "windowBackground = true paints the whole window";
        status = {
          api = {
            order = 100;
            label = "api";
            status = "ready";
          };
          db = {
            order = 200;
            label = "db";
            status = "ready";
          };
        };
      };
      clearScreen = false;
      margin.top = 1;
      margin.bottom = 1;
      windowBackground = true;
      description.text = "Every cell, gutter, and line remainder carries the background.";
    };
  };

  # `nix run .#example-themes` — a mini motd per theme, background-filled
  # so the palettes read as intended.
  themeMotd = theme: {
    inherit theme;
    project = theme;
    header = {
      tagline.text = "theme ${theme}";
      status.ready = {
        status = "ok";
      };
    };
    clearScreen = false;
    margin.y = 2;
    maxWidth = 60;
    windowBackground = true;
    description.text = "The quick brown fox jumps over the lazy dog.";
    commandCatalog.build = {
      description = "accent on commands";
      exec = "just build";
      group = "general";
    };
    commands = [ "build" ];
  };
in
{
  inherit
    commands
    motd
    menu
    motdDemos
    themeMotd
    ;
}
