{
  description = "Complete Prelude consumer reference";

  inputs = {
    prelude.url = "github:darkmatter/prelude";
    nixpkgs.follows = "prelude/nixpkgs";
    flake-parts.follows = "prelude/flake-parts";
  };

  outputs =
    inputs@{ flake-parts
    , prelude
    , ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      # 1. Import flake-parts mdule
      imports = [ prelude.flakeModules.default ];

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      # Configure - use optional helpers
      prelude = {
        theme = "phosphor";
        project = "prelude-reference";

        motd = {
          enable = true;
          clearScreen = false;
          windowBackground = true;

          header = {
            tagline.text = "complete downstream configuration";
            status.reference = {
              label = "example";
              status = "ready";
            };
          };

          description.text = "A runnable reference for Prelude's MOTD, menu, docs viewer, prompt, and package-backed commands.";

          env = [
            {
              label = "nix";
              probe = "nix --version | awk '{print $NF}'";
            }
            {
              label = "system";
              value = "flake-parts";
            }
          ];

          recipes.first-run = {
            title = "build and run the example";
            steps = [
              { command = "nix build .#hello"; }
              { command = "nix run .#hello"; }
            ];
          };
        };

        menu = {
          enable = true;
          placeholder = "Search reference commands…";
        };

        docs.pages = [
          { text = ./docs/getting-started.md; }
          { text = ./docs/customization.md; }
        ];

        prompt.enable = true;
      };

      perSystem =
        { config, pkgs, ... }:
        let
          hello = pkgs.writeShellApplication {
            name = "hello";
            text = ''
              echo "hello from the Prelude reference example"
            '';
          };
          helloApp = {
            type = "app";
            program = pkgs.lib.getExe hello;
          };
          packages = {
            inherit hello;
            default = hello;
          };
          apps = {
            hello = helloApp;
            default = helloApp;
          };
          checks.hello = pkgs.runCommand "reference-hello-check" { nativeBuildInputs = [ hello ]; } ''
            hello > "$out"
            grep -q "Prelude reference example" "$out"
          '';
        in
        {
          inherit packages apps checks;

          prelude.commands.hello = prelude.lib.fromPkg packages.hello {
            description = "run the example application";
            motd = 1;
          };

          devShells.default = pkgs.mkShell {
            packages = [ config.packages.prelude ];
            shellHook = ''
              export STARSHIP_CONFIG=${config.packages.prompt}
              motd >&2
            '';
          };
        };
    };
}
