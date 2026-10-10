# Dogfood configuration — the same flake-parts shape a consumer gets from
# `nix run github:darkmatter/prelude -- wizard`. Imported next to the module:
#
#   imports = [ prelude.flakeModules.default ./prelude.nix ];
#
# Component-specific detail lives under nix/ so this file stays the thin
# identity + catalogue surface users expect.
{
  self,
  lib,
  ...
}: let
  # Side-eval Prelude's public options so docs generation does not close over
  # the live flake-parts option tree (cycles / noise). Same modules as docs-sync.
  preludeOptionsEval = lib.evalModules {
    modules = [
      (self + /src/prelude/options/shared.nix)
      (self + /src/prelude/options/motd.nix)
      (self + /src/prelude/options/menu.nix)
      (self + /src/prelude/options/portal.nix)
      (self + /src/prelude/options/docs.nix)
      (self + /src/prelude/options/prompt.nix)
      (self + /src/prelude/options/workspace.nix)
    ];
  };
in {
  prelude = {
    theme = "minted";
    colorProfile = "truecolor";
    project = "prelude";

    prompt.enable = true;
    workspace.enable = true;
    # prompt.settings = {
    #   # format = "\n$directory$git_branch$character";
    #   # add_newline =  true;
    # };
    menu.enable = true;

    # Dogfood the launcher. prelude itself ships no servers, so the catalogue
    # points at its own docs site plus a local example, which is enough to
    # exercise up / down / gated rendering in both front ends.
    portal = {
      enable = true;
      apps = {
        docs = {
          description = "prelude documentation";
          order = 10;
          environments = {
            local.url = "http://127.0.0.1:8000";
            public.url = "https://darkmatter.github.io/prelude";
          };
        };
      };
    };
    menu.just.enable = true;

    # --------------------------------------------------------
    # commands
    # --------------------------------------------------------

    # If exec is omitted, it is the key's last word. The `motd` command
    # already exists in the shell. Repository tools use `:` keys, which stay
    # reachable only through `x`, so no `prelude:wizard` lands on PATH.
    commands.motd = {
      description = "reprint the welcome banner";
    };
    commands."prelude:previews" = {
      description = "build render checks and print their output to inspect the command catalogue, title fonts, and feature demos without opening an interactive surface";
      exec = "prelude-previews";
      group = "prelude";
      details = ''
        Build the selected render checks and print each resulting preview. Leave the argument line empty to inspect every render check, or choose individual checks to focus on the menu, titles, or the feature demos.
      '';
      usage = "x prelude:previews [check ...]";
      examples = [
        "x prelude:previews menu-list-renders"
        "x prelude:previews titles-command-renders examples-render"
      ];
      args = [
        {
          token = "<check>";
          description = "Render check name, or several names separated by spaces; leave empty to build every preview. The suggested choices cover the command catalogue, title fonts, and feature demos.";
          options = [
            "menu-list-renders"
            "titles-command-renders"
            "examples-render"
          ];
        }
      ];
    };
    commands."prelude:wizard" = {
      description = "run the interactive setup wizard to choose a title, theme, and command catalogue, then write a consumer's Prelude sidecar without replacing flake.nix";
      exec = "nix run . -- wizard";
      group = "prelude";
      motd = 0;
      details = ''
        Generate a Prelude sidecar and a sibling title.txt for a consumer project. Choose a different output path when experimenting in this repository so the wizard does not replace our own prelude.nix.
      '';
      usage = "x prelude:wizard [--output path] [--recipe path]";
      examples = ["x prelude:wizard --output .work/prelude.nix"];
      args = [
        {
          token = "--output";
          description = "Destination path for the generated Nix sidecar; the wizard writes title.txt beside it. Use .work/prelude.nix to try the wizard without replacing this repository's own configuration.";
          default = "prelude.nix";
          options = [".work/prelude.nix"];
        }
        {
          token = "--recipe";
          description = "Path to a Nix title recipe used to prefill the title text and font before the interactive wizard opens; leave this option unset to choose both in the wizard.";
        }
      ];
    };
    commands.build = {
      description = "build a Prelude flake output, such as the command menu, MOTD, docs viewer, or a feature demo, with optional build logs and result-link control";
      exec = "nix build";
      details = ''
        Build a flake output with Nix. Choose one of the component or demo outputs below, or enter another installable manually; leaving the target empty builds the default package.
        Use --no-link to avoid replacing the result symlink, and --print-build-logs to inspect compiler output while a package builds.
      '';
      usage = "x build [target] [--no-link] [--print-build-logs]";
      examples = [
        "x build .#prelude-menu --no-link"
        "x build .#prelude-motd --print-build-logs"
      ];
      args = [
        {
          token = "<target>";
          description = "Flake installable to build; choose a component or demo below, or type a target. Leave empty for the default package.";
          default = ".";
          options = [
            ".#prelude-motd"
            ".#prelude-menu"
            ".#prelude-docs"
            ".#example-themes"
            ".#example-default"
          ];
        }
        {
          token = "--no-link";
          description = "Keep the existing result symlink untouched while Nix builds the selected output.";
          boolean = true;
        }
      ];
    };
    # docs
    commands."sync-docs" = {
      description = "regenerate option and showcase markdown";
      exec = "docs-sync";
    };
    commands."record-docs" = {
      description = "record stale VHS showcases and sync docs";
      exec = "docs-record";
    };

    commands."prelude:workspace" = {
      description = "launch a Bash workspace with Prelude keyboard chords, movable menu and docs panes, and an optional Starship prompt";
      exec = "prelude-workspace";
      group = "prelude";
      details = ''
        Open an isolated Bash with a movable Prelude menu or docs pane. Alt+X toggles the command menu; Ctrl+] cycles pane layouts so you can inspect description wrapping at different widths.
        The fixed `prelude $` prompt is the default. Enable --starship to use the themed Prelude prompt and its navigation keymap instead.
      '';
      usage = "x prelude:workspace [--starship]";
      examples = ["x prelude:workspace --starship"];
      args = [
        {
          token = "--starship";
          description = "Use the themed Prelude Starship prompt instead of the fixed `prelude $` prompt, preserving its navigation keymap while the command menu and docs panes move between floating and split layouts.";
          boolean = true;
        }
      ];
    };

    # `demos` runs the tour and also opens its subcommands (`x demos themes`).
    commands.demos = {
      description = "tour every feature demo";
      exec = "nix run .#examples";
      motd = 3;
    };
    commands."demos titles" = {
      description = "inspect rendered titles";
      exec = "prelude-title-previews prelude";
    };
    commands."demos themes" = {
      description = "render a mini motd per theme";
      exec = "nix run .#example-themes";
    };
    commands."demos defaults" = {
      description = "preview MOTD from stock setup wizard presets";
      exec = "nix run .#example-default";
    };
    commands."gen" = {
      description = "run generation tasks";
      exec = ''
        sync-docs
        record-docs
      '';
    };

    docs = {
      nixosOptions = {
        options = {
          inherit (preludeOptionsEval.options) prelude;
        };
        transformOptions = option: option // {declarations = [];};
      };
      # mdSplit → { title = "README"; text; children }; docs.nix names the
      # preamble child after project and attaches FIGlet via rootReadme.
      rootReadme = self + /README.md;
      pages = [
        (self.lib.mdSplit (self + /README.md))
        {text = self + /docs/this-shell.md;}
        {text = self + /docs/commands.md;}
        {text = self + /docs/your-own-repo.md;}
        {text = self + /docs/configuration.md;}
        {text = self + /docs/see-also.md;}
        {
          generate = "nixosOptions";
          title = "Options";
        }
      ];
    };

    motd = {
      enable = true;
      border = false;
      # Layout (maxWidth, transparent chrome, margin/padding, statusHint) comes
      # from shared defaults; only project identity and probes stay here.
      title = {
        text = self + /nix/internal/title.txt;
      };

      header = {
        tagline = {
          text = "Devshell UI for Nix flakes";
          # subtitle = "MOTD, command menu, docs viewer, and prompt from one flake-parts module";
        };
        statusHint = {
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
        You are inside Prelude's own devshell — the banner, menu, docs, and prompt around you are built by this repo from `prelude.nix`, the same way a downstream project would. Run `x` to browse every command, or `docs` for the guides — including how to set up Prelude in your own repo.
      '';

      env = [];

      # Commands shown in Getting Started are selected via `commands.<name>.motd`
      # (sort order) — see nix/internal/prelude.nix. The menu component is always
      # listed as bare `x` when enabled. Recipes are separate multi-step workflows.

      # recipes.your-own-repo = {
      #   order = 100;
      #   title = "set up prelude in your own repo";
      #   steps = [
      #     {comment = "generate config with the setup wizard";}
      #     {command = "nix run github:darkmatter/prelude -- wizard";}
      #     {comment = "full walkthrough: docs, page \"Your own repo\"";}
      #   ];
      # };
    };

    # Preferred command-group order. Commands without a group list above every
    # heading, then Prelude's own group; unlisted groups follow alphabetically.
    sort.groups = [
      "go"
      "ts"
    ];
  };

  # Package-backed commands derive both their executable and runtime closure.
  perSystem = {
    pkgs,
    config,
    ...
  }: {
    prelude.commands = {
      # `:` keys stay reachable only through `x` (`x go:test`), so no
      # wrapper named after the tool shadows `go` itself; `group` places them
      # under a `go` heading. fromPkg derives the canonical `go test …`
      # invocation and carries Go onto PATH; no extra executable is generated.
      "go:test" =
        self.lib.fromPkg pkgs.go {
          arguments = [
            "test"
            "-C"
            "src"
            "./..."
          ];
          description = "run the Go unit tests";
        }
        // {group = "go";};
      "go:vet" =
        self.lib.fromPkg pkgs.go {
          arguments = [
            "vet"
            "-C"
            "src"
            "./..."
          ];
          description = "run Go static analysis";
        }
        // {group = "go";};
      # TypeScript API (ts/). The devshell's PRELUDE_LIB lets the FFI tests
      # drive the real Go surfaces; `bun test` itself needs no install.
      "ts:test" =
        self.lib.fromPkg pkgs.bun {
          arguments = [
            "--cwd"
            "ts"
            "test"
          ];
          description = "run the TypeScript API tests";
        }
        // {group = "ts";};
      "ts:typecheck" =
        self.lib.mkCommand {
          # `--cwd=ts`, not `--cwd ts`: bun run misreads the spaced form.
          command = "bun --cwd=ts install --frozen-lockfile && bun --cwd=ts run typecheck";
          description = "type-check the TypeScript API, including its type tests";
        }
        // {group = "ts";};
      "ts:sync" = {
        description = "regenerate the TypeScript API's themes, defaults, and fixtures";
        exec = "ts-sync";
        group = "ts";
      };
      check = self.lib.mkCommand {
        command = "nix flake check";
        description = "build + render smoke tests";
      };
      fmt = self.lib.fromPkg config.treefmt.build.wrapper {
        arguments = ["."];
        description = "format nix sources";
      };
    };
  };
}
