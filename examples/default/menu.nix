# Showcase: configure the command menu and its command catalogue.
{ self, ... }:
{
  prelude = {
    menu.enable = true;
    prompt.enable = true;

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

      usage = "menu build .#motd";
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
  };

  # Sorting
  sort.groups = [
    "develop"
    "go"
    "docs"
    "demos"
  ];

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
