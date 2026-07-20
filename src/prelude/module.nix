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
#       imports = [ prelude.flakeModules.default ];
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
#           packages = [ config.packages.prelude ];
#           shellHook = ''
#             motd
#           '';
#         };
#       };
#     };
#
# The outer function receives static args via flake-parts' `importApply`
# (see flake.nix); consumers should import the applied module from
# `flakeModules.default`, not this file directly.
{ localFlake, flake-parts-lib }:
{ lib, config, ... }:
let
  # Currently unused; kept so the exported module can reference the prelude
  # flake itself (per the flake-parts importApply pattern) without a
  # breaking signature change later.
  _unusedLocalFlake = localFlake;

  cfg = config.prelude;
  sortCfg = cfg.sort;
  docsEnabled = cfg.docs.pages != [ ];
  internalShortcuts = plib.componentShortcuts {
    motd = cfg.motd.enable;
    menu = cfg.menu.enable;
    docs = docsEnabled;
  };

  mkMotd = import ./motd.nix;
  mkTitle = import ./title-generator.nix;
  mkTitlePreviews = import ./title-previews.nix;
  mkMenu = import ./menu.nix;
  mkDocs = import ./docs.nix;
  mkPrompt = import ./prompt.nix;
  plib = import ./lib.nix { inherit lib; };
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
        description = "System-specific project commands, including package-backed commands created with prelude.lib.fromPkg.";
      };
    }
  );

  config = {
    # Prelude owns its navigation commands and default accelerators. Consumers
    # can still override any field explicitly, while project command catalogues
    # stay focused on lifecycle actions such as serve, build, test, and install.
    prelude.commands = lib.mkMerge [
      (lib.mkIf cfg.menu.enable {
        menu = {
          description = lib.mkDefault "open the interactive command menu";
          exec = lib.mkDefault "menu";
          key = lib.mkDefault "m";
        };
        help = {
          description = lib.mkDefault "show Prelude command help";
          exec = lib.mkDefault "menu help";
          key = lib.mkDefault "h";
        };
      })
      (lib.mkIf docsEnabled {
        docs = {
          description = lib.mkDefault "browse project documentation";
          exec = lib.mkDefault "docs";
          key = lib.mkDefault "d";
        };
      })
    ];

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
            symlinkJoin
            figlet
            jq
            nix
            formats
            ;
        };

        motdRenderConfig =
          generatorConfig cfg.motd
          // {
            commandCatalog = commands;
            commandGroupOrder = sortCfg.groups;
            shortcuts = internalShortcuts;
          };
        motdBin = mkMotd deps motdRenderConfig;
        titlePkg = mkTitle deps;
        titlePreviewsPkg = mkTitlePreviews deps;
        setupPkg = pkgs.writeShellApplication {
          name = "setup";
          runtimeInputs = [ titlePkg ];
          text = ''
            if [ "''${1:-}" = "--help" ] || [ "''${1:-}" = "-h" ]; then
              cat <<'EOF'
            usage: setup [--recipe path] [-o path]

            Interactively generate a ready-to-use Prelude configuration.
            The UI renders on stderr. Writes the Nix config to -o and a sibling
            title.txt next to it (e.g. prelude.nix + title.txt).

              -o, --output path  write the generated config (default: prelude.nix)
              --recipe path      prefill title text and font from a Nix recipe
            EOF
              exit 0
            fi
            exec prelude-title --wizard "$@"
          '';
          meta.description = "Interactively generate a Prelude project configuration";
        };

        motdPkg = pkgs.symlinkJoin {
          name = "motd";
          # Command-backed MOTD rows remain runnable when packages.motd is used
          # directly by carrying the menu and its generated wrappers. Built-in
          # navigation aliases ride along with the menu when enabled; otherwise
          # the MOTD package carries them.
          paths = [
            motdBin
            titlePkg
            titlePreviewsPkg
            setupPkg
          ]
          ++ lib.optional cfg.menu.enable menuPkg
          ++ lib.optionals (!cfg.menu.enable) shortcutWrappers;
          passthru = {
            inherit motdRenderConfig;
            commandNames = map (command: command.name) selectedMotdCommands;
            commandInvocations = map (command: command.command) selectedMotdCommands;
            commandWrappers = lib.optionals cfg.menu.enable menuPkg.commandWrappers;
            shortcutAliases = if cfg.menu.enable then menuPkg.shortcutAliases else shortcutAliases;
            shortcutWrappers = if cfg.menu.enable then menuPkg.shortcutWrappers else shortcutWrappers;
          };
          meta = {
            inherit (motdBin.meta) description;
            mainProgram = "motd";
          };
        };

        menuBin = mkMenu deps (
          generatorConfig cfg.menu
          // {
            inherit commands;
            groupOrder = sortCfg.groups;
          }
        );

        commandEntries = plib.normalizeCommandEntries commands;
        commandNames = map (entry: entry.name) commandEntries;
        selectedMotdCommands = plib.selectCommands commandEntries;
        commandRuntimePackages = lib.unique (
          lib.concatMap (entry: entry.raw.runtimePackages) commandEntries
        );

        # Menu entries are devshell commands too. A command whose `exec` starts
        # with its own name asserts "this command already exists on PATH"
        # (motd, docs, previews…); every other command gets a generated wrapper
        # that delegates to the menu fast path (`menu <name> …`) so direct and
        # interactive invocation share one execution contract.
        needsWrapper = entry: builtins.head (lib.splitString " " entry.run) != entry.name;
        # Colon-grouped entries are catalogue identity only. Never turn them
        # into shell executables: the complete key stays public through x while
        # its first colon derives menu presentation.
        wrappedCommandEntries = lib.filter (entry: !entry.grouped && needsWrapper entry) commandEntries;
        commandWrappers =
          let
            wrapped = wrappedCommandEntries;
          in
          assert lib.assertMsg
            (
              !lib.any
                (
                  entry:
                  lib.elem entry.name [
                    "menu"
                    "x"
                  ]
                )
                wrapped
            )
            "prelude: ungrouped commands named \"menu\" or \"x\" cannot receive wrappers because Prelude owns those entrypoints";
          map
            (
              entry:
              # writeTextFile rather than writeShellApplication: public command
              # keys may contain ":" (valid in bin/ entries, unsafe in store names).
              pkgs.writeTextFile {
                name = "prelude-command-${lib.replaceStrings [ ":" ] [ "-" ] entry.name}";
                executable = true;
                destination = "/bin/${entry.name}";
                text = ''
                  #!${pkgs.runtimeShell}
                  exec ${lib.getExe menuBin} ${lib.escapeShellArg entry.name} "$@"
                '';
              }
            )
            wrapped;

        # Built-in navigation aliases are PATH wrappers so every rendered chip
        # is runnable. Resolve targets to absolute store paths so shell builtins
        # cannot shadow Prelude commands.
        shortcutEntries = internalShortcuts;
        shortcutAliases = map (s: s.alias) shortcutEntries;
        entriesByName = lib.listToAttrs (map (entry: lib.nameValuePair entry.name entry) commandEntries);
        resolveShortcutTarget =
          command:
          if entriesByName ? ${command} then
            let
              entry = entriesByName.${command};
              head = builtins.head (lib.splitString " " entry.run);
            in
            if needsWrapper entry then
              "${lib.getExe menuBin} ${lib.escapeShellArg entry.name}"
            else if head == "menu" && cfg.menu.enable then
              lib.getExe menuBin
            else if head == "docs" && docsEnabled then
              lib.getExe docsBin
            else if head == "motd" && cfg.motd.enable then
              lib.getExe motdBin
            else
              lib.escapeShellArg head
          else if command == "menu" && cfg.menu.enable then
            lib.getExe menuBin
          else if command == "docs" && docsEnabled then
            lib.getExe docsBin
          else if command == "motd" && cfg.motd.enable then
            lib.getExe motdBin
          else
            lib.escapeShellArg command;
        shortcutWrappers = map
          (
            s:
            pkgs.writeTextFile {
              # Alias may be `?` or other non-store-safe glyphs; sanitize the
              # derivation name while keeping the bin/ entry exact.
              name = "prelude-shortcut-${lib.replaceStrings [ "?" ":" "/" " " ] [ "q" "-" "-" "-" ] s.alias}";
              executable = true;
              destination = "/bin/${s.alias}";
              text = ''
                #!${pkgs.runtimeShell}
                exec ${resolveShortcutTarget s.command} "$@"
              '';
            }
          )
          shortcutEntries;

        menuPkg = pkgs.symlinkJoin {
          name = "menu";
          paths = [
            menuBin
          ]
          ++ commandWrappers
          ++ shortcutWrappers
          ++ commandRuntimePackages
          ++ lib.optional docsEnabled docsPkg;
          passthru = {
            inherit
              commandNames
              commandWrappers
              commandRuntimePackages
              shortcutAliases
              shortcutWrappers
              ;
            commandInvocations = map (entry: entry.invocation) commandEntries;
            xInvocations = map (entry: entry.xInvocation) commandEntries;
            commandWrapperNames = map (entry: entry.name) wrappedCommandEntries;
          };
          meta = {
            inherit (menuBin.meta) description;
            mainProgram = "menu";
          };
        };

        docsBin = mkDocs deps (generatorConfig cfg.docs);
        docsPkg =
          if cfg.motd.enable || cfg.menu.enable then
            docsBin
          else
            pkgs.symlinkJoin {
              name = "docs";
              paths = [ docsBin ] ++ shortcutWrappers;
              passthru = { inherit shortcutAliases shortcutWrappers; };
              meta = {
                inherit (docsBin.meta) description;
                mainProgram = "docs";
              };
            };
        promptPkg = mkPrompt deps (generatorConfig cfg.prompt // { shortcuts = internalShortcuts; });

        # Canonical devshell package. Component packages already compose their
        # enabled descendants (motd -> menu -> docs), so select only the
        # outermost enabled component and add prompt runtimes when requested.
        preludeComponentPaths =
          lib.optional cfg.motd.enable motdPkg
          ++ lib.optional (!cfg.motd.enable && cfg.menu.enable) menuPkg
          ++ lib.optional (!cfg.motd.enable && !cfg.menu.enable && docsEnabled) docsPkg;
        promptRuntimePackages = lib.optionals cfg.prompt.enable [
          pkgs.starship
          pkgs.blesh
        ];
        preludePkg = pkgs.symlinkJoin {
          name = "prelude";
          paths = preludeComponentPaths ++ promptRuntimePackages;
          postBuild = lib.optionalString cfg.prompt.enable ''
            mkdir -p "$out/nix-support"
            cat > "$out/nix-support/setup-hook" <<'EOF'
            # Initialize Prelude's enhanced prompt only in the interactive Bash
            # process created by `nix develop`; direnv evaluation stays inert.
            case "$-" in
              *i*)
                if [ -n "''${BASH_VERSION-}" ]; then
                  source ${pkgs.blesh}/share/blesh/ble.sh
                  eval "$(${lib.getExe pkgs.starship} init bash)"
                fi
                ;;
            esac
            EOF
          '';
          passthru = {
            inherit preludeComponentPaths promptRuntimePackages;
          }
          // lib.optionalAttrs cfg.prompt.enable {
            prompt = promptPkg;
          };
          meta = {
            description = "Prelude devshell UI and its enabled runtime dependencies";
          }
          // lib.optionalAttrs cfg.motd.enable {
            mainProgram = "motd";
          };
        };

        mkApp = pkg: {
          type = "app";
          program = pkgs.lib.getExe pkg;
        };
      in
      lib.mkMerge [
        {
          # Add this single package to a devshell to receive every enabled
          # Prelude component and its runtime dependencies.
          packages.prelude = preludePkg;
        }
        (lib.mkIf cfg.motd.enable {
          packages.motd = motdPkg;
          packages.title = titlePkg;
          packages.title-previews = titlePreviewsPkg;
          packages.setup = setupPkg;
          apps.motd = mkApp motdPkg;
          apps.title = mkApp titlePkg;
          apps.title-previews = mkApp titlePreviewsPkg;
          apps.setup = mkApp setupPkg;
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
