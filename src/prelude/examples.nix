# Runnable feature demos for the prelude components.
#
# Motd demos become packages/apps named `example-<name>`:
#
#   nix run .#example-minimal
#
# `nix run .#example-motd` and `nix run .#example-menu` render the prelude
# repo's own dogfood config; `nix run .#example-default` previews stock wizard
# presets; `nix run .#example-themes` pages the current MOTD through every
# theme. The `minimal` and `surface` demos below exercise specific rendering
# features (explicit styling, bounded cards) independent of any repo config.
# Render everything:
#
#   nix run .#examples
#
# Demos disable clearScreen and the top margin so they can render one after
# another.
let
  # Demo commands, ported from the design's lib/devshell.ts + commands.ts.
  # Exercises: colon-derived groups/labels, exec-vs-name, details/usage/examples,
  # optional args with suggestions, boolean flags, required positional args,
  # and free text.
  commands = {
    "general:clean" = {
      description = "remove build artifacts & caches";
      exec = "rm -rf .next .turbo node_modules/.cache";
    };
    dev = {
      description = "start the dev server with hot reload";
      exec = "pnpm dev";
      motd = 1;

      usage = "x dev --port 3000";
      details = "Boots a development server that watches the source tree and hot-reloads modules as files change. Binds to 127.0.0.1:3000 by default; override with --port and --host.";
      examples = [
        "x dev --port 8080"
        "x dev --host 0.0.0.0"
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
    };
    test = {
      description = "run the unit test suite";
      exec = "pnpm test";
      motd = 2;
    };
    "database:up" = {
      description = "start postgres & redis in the background";
      exec = "docker compose up -d db redis";
    };
    "database:migrate" = {
      description = "apply pending schema migrations";
      exec = "drizzle-kit migrate";
    };
    "ops:deploy" = {
      description = "ship the current build to production";
      exec = "vercel deploy";

      usage = "x ops:deploy --alias staging";
      details = "Uploads the most recent production build and promotes it to the live environment. Deploys are atomic: traffic switches only after the new release passes its health checks.";
      examples = [
        "x ops:deploy --dry-run"
        "x ops:deploy --alias staging"
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
    "ops:push" = {
      description = "publish the current branch to the remote";
      exec = "git push";

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

  # Reference MOTD config (acme-web fixture). No longer wired to
  # `example-motd` — that demo uses the prelude repo's own config now.
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
    recipes = {
      clean-local-stack = {
        title = "spin up a clean local stack";
        steps = [
          {comment = "start postgres + redis first";}
          {command = "just db:up";}
          {command = "just db:migrate && just db:seed";}
          {command = "just dev";}
        ];
      };
      ship-hotfix = {
        title = "ship a hotfix to production";
        steps = [
          {command = "git checkout -b fix/login";}
          {comment = "verify before deploying";}
          {command = "just test && just build";}
          {command = "just deploy";}
        ];
      };
    };
  };

  # Reference menu config (acme-web fixture). No longer wired to
  # `example-menu` — that demo uses the prelude repo's own config now.
  menu = {
    project = "acme-web";
    inherit commands;
    groupOrder = [
      "develop"
      "general"
      "database"
      "ops"
    ];
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

    # Bounded opaque card + header status.
    surface = {
      project = "surface";
      header = {
        tagline.text = "the card stays visually bounded within the terminal";
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
      background = true;
      description.text = "Only the bounded card paints a background; surrounding cells stay terminal-transparent.";
    };
  };
in {
  inherit
    commands
    motd
    menu
    motdDemos
    ;
}
