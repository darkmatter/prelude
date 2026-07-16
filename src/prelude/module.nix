# flake-parts module: the prelude devshell UI suite.
#
#   prelude.motd    — devshell welcome banner
#   prelude.menu    — interactive command menu
#   prelude.docs    — Markdown project docs viewer
#   prelude.prompt  — themed starship config (packages.prompt = starship.toml)
#
# Shared config covers theme/palette, project identity, and a flat command
# catalogue. MOTD guidance and docs content are authored independently.
# Options are declared in ./options/{shared,motd,menu,docs}.nix.
#
#   outputs = { prelude, flake-parts, ... }@inputs:
#     flake-parts.lib.mkFlake { inherit inputs; } {
#       imports = [ prelude.flakeModules.prelude ];
#
#       prelude = {
#         theme = "phosphor";
#         project = "acme-web";
#         motd.header.tagline.text = "everything you need to build, test & ship";
#
#         commands.dev = {
#           description = "start the dev server with hot reload";
#           exec = "pnpm dev";
#           group = "develop";
#           key = "d";
#           order = 100;
#         };
#
#         docs.pages = [
#           { text = ./docs/getting-started.md; }
#         ];
#
#         motd.enable = true;
#         menu.enable = true;
#       };
#
#       perSystem = { pkgs, config, ... }: {
#         devShells.default = pkgs.mkShell {
#           packages = [
#             config.packages.motd
#             config.packages.menu
#             config.packages.docs
#           ];
#           shellHook = ''
#             motd
#           '';
#         };
#       };
#     };
#
# The outer function receives static args via flake-parts' `importApply`
# (see flake.nix); consumers should import the applied module from
# `flakeModules.prelude`, not this file directly.
{ localFlake, flake-parts-lib }:
{ lib, config, ... }:
let
  # Currently unused; kept so the exported module can reference the prelude
  # flake itself (per the flake-parts importApply pattern) without a
  # breaking signature change later.
  _unusedLocalFlake = localFlake;

  cfg = config.prelude;
  docsEnabled = cfg.docs.pages != [ ];

  mkMotd = import ./motd.nix;
  mkTitle = import ./title-generator.nix;
  mkTitlePreviews = import ./title-previews.nix;
  mkMenu = import ./menu.nix;
  mkDocs = import ./docs.nix;
  mkPrompt = import ./prompt.nix;
  optionTypes = import ./option-types.nix { inherit lib; };

  # Shared config threaded into every generator.
  shared = {
    inherit (cfg)
      theme
      palette
      colorProfile
      project
      ;
  };

  # Generator config is the evaluated option set minus module-only activation.
  # Passing the complete set avoids a second field list that can silently drift
  # when options are added.
  generatorConfig = component: shared // removeAttrs component [ "enable" ];
in
{
  imports = [
    ./options/shared.nix
    ./options/motd.nix
    ./options/menu.nix
    ./options/docs.nix
    ./options/prompt.nix
  ];

  options.perSystem = flake-parts-lib.mkPerSystemOption (
    { lib, ... }: {
      options.prelude.commands = lib.mkOption {
        type = lib.types.attrsOf optionTypes.commandType;
        default = { };
        description = "System-specific commands, including package-backed commands created with prelude.lib.mkCommand.";
      };
    }
  );

  config = {
    # Advertise the menu command by default; an explicit list replaces this.
    prelude.motd.commands = lib.mkIf cfg.menu.enable (lib.mkDefault [ "menu" ]);

    # The prompt mirrors the MOTD footer chips by default; an explicit list
    # (or [ ]) replaces this.
    prelude.prompt.shortcuts = lib.mkDefault cfg.motd.shortcuts;

    perSystem =
      { pkgs, config, ... }:
      let
        commands = lib.recursiveUpdate cfg.commands config.prelude.commands;
        deps = {
          inherit (pkgs)
            lib
            writeShellApplication
            writeText
            buildGoModule
            figlet
            jq
            nix
            formats
            ;
        };

        motdBin = mkMotd deps (generatorConfig cfg.motd // { commandCatalog = commands; });
        titlePkg = mkTitle deps;
        titlePreviewsPkg = mkTitlePreviews deps;

        motdPkg = pkgs.symlinkJoin {
          name = "motd";
          # Command-backed MOTD rows remain runnable when packages.motd is used
          # directly by carrying the menu and its generated wrappers.
          paths = [
            motdBin
            titlePkg
            titlePreviewsPkg
          ]
          ++ lib.optional cfg.menu.enable menuPkg;
          passthru = {
            commandNames = cfg.motd.commands;
            commandWrappers = lib.optionals cfg.menu.enable menuPkg.commandWrappers;
          };
          meta = {
            inherit (motdBin.meta) description;
            mainProgram = "motd";
          };
        };

        menuBin = mkMenu deps (generatorConfig cfg.menu // { inherit commands; });

        commandEntries = lib.mapAttrsToList (name: command: { inherit name command; }) commands;
        commandNames = map ({ name, ... }: name) commandEntries;
        commandRuntimePackages = lib.unique (
          lib.concatMap ({ command, ... }: command.runtimePackages) commandEntries
        );

        # Menu entries are devshell commands too. A command whose `exec` starts
        # with its own name asserts "this command already exists on PATH"
        # (motd, docs, previews…); every other command gets a generated wrapper
        # that delegates to the menu fast path (`menu <name> …`) so direct and
        # interactive invocation share one execution contract.
        commandWrappers =
          let
            needsWrapper =
              { name, command }:
              command.exec != null && builtins.head (lib.splitString " " command.exec) != name;
            wrapped = lib.filter needsWrapper commandEntries;
          in
          assert lib.assertMsg (
            !lib.any ({ name, ... }: name == "menu") wrapped
          ) "prelude: a command named \"menu\" whose `exec` is not `menu …` would shadow the menu itself";
          map (
            { name, ... }:
            # writeTextFile rather than writeShellApplication: command names may
            # contain ":" (valid in bin/ entries, invalid in store names).
            pkgs.writeTextFile {
              name = "prelude-command-${lib.replaceStrings [ ":" ] [ "-" ] name}";
              executable = true;
              destination = "/bin/${name}";
              text = ''
                #!${pkgs.runtimeShell}
                exec ${lib.getExe menuBin} ${lib.escapeShellArg name} "$@"
              '';
            }
          ) wrapped;

        menuPkg = pkgs.symlinkJoin {
          name = "menu";
          paths = [ menuBin ] ++ commandWrappers ++ commandRuntimePackages;
          passthru = {
            inherit commandNames commandWrappers commandRuntimePackages;
          };
          meta = {
            inherit (menuBin.meta) description;
            mainProgram = "menu";
          };
        };

        docsPkg = mkDocs deps (generatorConfig cfg.docs);
        promptPkg = mkPrompt deps (generatorConfig cfg.prompt);

        mkApp = pkg: {
          type = "app";
          program = pkgs.lib.getExe pkg;
        };
      in
      lib.mkMerge [
        (lib.mkIf cfg.motd.enable {
          packages.motd = motdPkg;
          packages.title = titlePkg;
          packages.title-previews = titlePreviewsPkg;
          apps.motd = mkApp motdPkg;
          apps.title = mkApp titlePkg;
          apps.title-previews = mkApp titlePreviewsPkg;
        })
        (lib.mkIf cfg.menu.enable {
          packages.menu = menuPkg;
          apps.menu = mkApp menuPkg;
        })
        (lib.mkIf docsEnabled {
          packages.docs = docsPkg;
          apps.docs = mkApp docsPkg;
        })
        # A config file, not a program — no app entry.
        (lib.mkIf cfg.prompt.enable {
          packages.prompt = promptPkg;
        })
      ];
  };
}
