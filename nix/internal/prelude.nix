# Dogfood configuration — the same flake-parts shape a consumer gets from
# `nix run github:darkmatter/prelude` / setup. Imported next to the module:
#
#   imports = [ prelude.flakeModules.default ./prelude.nix ];
#
# Component-specific detail lives under nix/ so this file stays the thin
# identity + catalogue surface users expect.
{ self, lib, ... }:
let
  # Side-eval Prelude's public options so docs generation does not close over
  # the live flake-parts option tree (cycles / noise). Same modules as docs-sync.
  preludeOptionsEval = lib.evalModules {
    modules = [
      (self + /src/prelude/options/shared.nix)
      (self + /src/prelude/options/motd.nix)
      (self + /src/prelude/options/menu.nix)
      (self + /src/prelude/options/docs.nix)
      (self + /src/prelude/options/prompt.nix)
    ];
  };
in
{

  prelude = {
    theme = "minted";
    colorProfile = "truecolor";
    project = "prelude";

    prompt.enable = true;
    menu.enable = true;

    # --------------------------------------------------------
    # commands
    # --------------------------------------------------------

    # If exec is omitted, it is inferred from the parsed command name. The
    # ungrouped `motd` and `previews` commands already exist in the shell.
    commands.motd = {
      description = "reprint the welcome banner";
    };
    commands.previews = {
      description = "build the render checks and show their output";
    };
    commands.wizard = {
      description = "run the interactive setup wizard";
      exec = "nix run .#setup";
      motd = 0;
    };
    commands.build = {
      description = "build a flake output";
      exec = "nix build";

      usage = "x build .#motd";
      args = [
        {
          token = "<target>";
          description = "flake output to build";
          options = [
            ".#motd"
            ".#menu"
            ".#docs"
            ".#example-themes"
          ];
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

    commands."demos:titles" = {
      description = "inspect rendered titles";
      exec = "prelude-title-previews prelude";
    };
    commands."demos" = {
      description = "tour every feature demo";
      exec = "nix run .#examples";
      motd = 3;
    };
    commands."demos:themes" = {
      description = "render a mini motd per theme";
      exec = "nix run .#example-themes";
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
        transformOptions = option: option // { declarations = [ ]; };
      };
      # mdSplit → { title = "README"; text; children }; docs.nix names the
      # preamble child after project and attaches FIGlet via rootReadme.
      rootReadme = self + /README.md;
      pages = [
        (self.lib.mdSplit (self + /README.md))
        { text = self + /docs/this-shell.md; }
        { text = self + /docs/commands.md; }
        { text = self + /docs/your-own-repo.md; }
        { text = self + /docs/configuration.md; }
        { text = self + /docs/see-also.md; }
        {
          generate = "nixosOptions";
          title = "Options";
        }
      ];
    };

    motd = {
      enable = true;
      title = {
        text = self + /nix/internal/title.txt;
        align = "center";
        style = "spine";
      };
      maxWidth = 120;
      windowBackground = false;
      background = false;
      clearScreen = true;
      # Bottom breathing room only when the terminal can afford it; short
      # windows drop it instead of scrolling the card away.
      margin = {
        bottom = 8;
        x = 4;
        minHeight = 40;
      };
      # padding.x = 4;
      # padding.bottom = 2;
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
        You are inside Prelude's own devshell — the banner, menu, docs, and prompt around you are built by this repo from `prelude.nix`, the same way a downstream project would. Run `menu` to browse every command, or `docs` for the guides — including how to set up Prelude in your own repo.
      '';

      env = [ ];

      # Commands shown in Getting Started are selected via `commands.<name>.motd`
      # (sort order) — see nix/internal/prelude.nix. `menu` is always listed bare when
      # enabled. Recipes are separate multi-step workflows.

      recipes.your-own-repo = {
        order = 100;
        title = "set up prelude in your own repo";
        steps = [
          { comment = "generate config with the setup wizard"; }
          { command = "nix run github:darkmatter/prelude"; }
          { comment = "full walkthrough: docs, page \"Your own repo\""; }
        ];
      };
    };

    # Preferred command-group order. Unlisted groups follow alphabetically;
    # Prelude's built-in navigation group remains first.
    sort.groups = [
      "develop"
      "go"
      "docs"
      "demos"
    ];
  };

  # Package-backed commands derive both their executable and runtime closure.
  perSystem =
    { pkgs, ... }:
    {
      prelude.commands = {
        # The first colon derives menu group/label while the complete key stays
        # public (`x go:test`). fromPkg derives the canonical `go test …`
        # invocation and carries Go onto PATH; no extra executable is generated.
        "test" = self.lib.fromPkg pkgs.go {

          arguments = [
            "test"
            "-C"
            "src"
            "./..."
          ];
          description = "run the Go unit tests";
          motd = 1;
        };
        "go:vet" = self.lib.fromPkg pkgs.go {

          arguments = [
            "vet"
            "-C"
            "src"
            "./..."
          ];
          description = "vet the Go sources";
        };
        check = self.lib.mkCommand {
          command = "nix flake check";
          description = "build + render smoke tests";
          motd = 2;
        };
        fmt = self.lib.fromPkg pkgs.nixfmt {
          arguments = [ "." ];
          description = "format nix sources";
        };
      };
    };
}
