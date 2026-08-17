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
{
  localFlake,
  flake-parts-lib,
}: {
  lib,
  config,
  ...
}: let
  # Currently unused; kept so the exported module can reference the prelude
  # flake itself (per the flake-parts importApply pattern) without a
  # breaking signature change later.
  _unusedLocalFlake = localFlake;

  cfg = config.prelude;
  sortCfg = cfg.sort;
  docsEnabled = cfg.docs.pages != [];
  internalShortcuts = plib.componentShortcuts {
    motd = cfg.motd.enable;
    menu = cfg.menu.enable;
    docs = docsEnabled;
  };

  mkMotd = import ./motd.nix;
  mkTitle = import ./title-generator.nix;
  mkTitlePreviews = import ./title-previews.nix;
  mkMenu = import ./menu.nix;
  mkPortal = import ./portal.nix;
  mkDocs = import ./docs.nix;
  mkPrompt = import ./prompt.nix;
  mkPromptStatus = import ./prompt-status.nix;
  mkShellInit = import ./shell-init.nix;
  plib = import ./lib.nix {inherit lib;};
  optionTypes = import ./option-types.nix {inherit lib;};

  # Shared config threaded into every generator.
  shared = {
    inherit
      (cfg)
      theme
      palette
      colorProfile
      project
      ;
  };

  # Generator config is the evaluated option set minus module-only activation.
  # Passing the complete set avoids a second field list that can silently drift
  # when options are added.
  generatorConfig = component: shared // removeAttrs component ["enable"];
in {
  imports = [
    ./options/shared.nix
    ./options/motd.nix
    ./options/menu.nix
    ./options/portal.nix
    ./options/docs.nix
    ./options/prompt.nix
  ];

  options.perSystem = flake-parts-lib.mkPerSystemOption (
    {lib, ...}: {
      options.prelude.commands = lib.mkOption {
        type = lib.types.attrsOf optionTypes.commandType;
        default = {};
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
        x = {
          description = lib.mkDefault "open the interactive command menu";
          exec = lib.mkDefault "x";
          key = lib.mkDefault "m";
        };
      })
      (lib.mkIf docsEnabled {
        docs = {
          description = lib.mkDefault "browse project documentation";
          exec = lib.mkDefault "docs";
          key = lib.mkDefault "d";
        };
      })
      (lib.mkIf cfg.portal.enable {
        portal = {
          description = lib.mkDefault "launch an app, with live health lights";
          exec = lib.mkDefault "portal";
          # `p` rather than a mnemonic for "web": the terminal front end is the
          # default, and `m`/`d` are already taken by the menu and docs.
          key = lib.mkDefault "p";
        };
      })
    ];

    perSystem = {
      pkgs,
      config,
      ...
    }: let
      commands = lib.recursiveUpdate cfg.commands config.prelude.commands;
      deps = {
        inherit
          (pkgs)
          lib
          writeShellApplication
          writeText
          runCommand
          nixosOptionsDoc
          symlinkJoin
          figlet
          jq
          nix
          formats
          ;
        # Downstream flakes may use a Nixpkgs whose default Go still trails
        # src/go.mod. Select the required toolchain instead of that alias.
        buildGoModule = pkgs.buildGo126Module;
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
        # Keep the flake output as `.#setup`, but avoid installing a generic
        # `setup` executable into consumers' shells. The stable installed name
        # is `prelude-setup`; the canonical user interface is `prelude setup`.
        name = "prelude-setup";
        runtimeInputs = [titlePkg];
        text = ''
          if [ "''${1:-}" = "--help" ] || [ "''${1:-}" = "-h" ]; then
            cat <<'EOF'
          usage: prelude setup [--recipe path] [-o path]

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
        paths =
          [
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
          shortcutAliases =
            if cfg.menu.enable
            then menuPkg.shortcutAliases
            else shortcutAliases;
          shortcutWrappers =
            if cfg.menu.enable
            then menuPkg.shortcutWrappers
            else shortcutWrappers;
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

      portalPkg = mkPortal deps (generatorConfig cfg.portal);

      commandEntries = plib.normalizeCommandEntries commands;
      # Resolve only after root and per-system command entries have merged.
      # A local server is a canonical `x` target, so its copyable start hint
      # must reuse the catalogue's shell-escaped dispatcher invocation.
      promptLocalServer = let
        configured = cfg.prompt.localServer;
      in
        if configured == null
        then null
        else let
          entry = lib.findFirst (candidate: candidate.name == configured.command) null commandEntries;
        in
          assert lib.assertMsg (
            entry != null
          ) "prelude.prompt.localServer.command must name a canonical prelude.commands key";
            configured // {start = entry.xInvocation;};
      commandNames = map (entry: entry.name) commandEntries;
      selectedMotdCommands = plib.selectCommands commandEntries;
      commandRuntimePackages = lib.unique (
        lib.concatMap (entry: entry.raw.runtimePackages) commandEntries
      );

      # Menu entries are devshell commands too. A command whose `exec` starts
      # with its own name asserts "this command already exists on PATH"
      # (motd, docs, previews…); every other command gets a generated wrapper
      # that delegates to the public `x` dispatcher so direct and interactive
      # invocation share one execution contract. Bare `menu` remains a
      # picker-only compatibility wrapper outside the catalogue.
      needsWrapper = entry: builtins.head (lib.splitString " " entry.run) != entry.name;
      # Colon-grouped entries are catalogue identity only. Never turn them
      # into shell executables: the complete key stays public through x while
      # its first colon derives menu presentation.
      wrappedCommandEntries = lib.filter (entry: !entry.grouped && needsWrapper entry) commandEntries;
      commandWrappers = let
        wrapped = wrappedCommandEntries;
        xBin = lib.getExe' menuBin "x";
      in
        assert lib.assertMsg
        (!lib.any (
          entry:
            lib.elem entry.name [
              "menu"
              "x"
            ]
        )
        wrapped)
        "prelude: ungrouped commands named \"menu\" or \"x\" cannot receive wrappers because Prelude owns those entrypoints";
          map (
            entry:
            # writeTextFile rather than writeShellApplication: public command
            # keys may contain ":" (valid in bin/ entries, unsafe in store names).
              pkgs.writeTextFile {
                name = "prelude-command-${lib.replaceStrings [":"] ["-"] entry.name}";
                executable = true;
                destination = "/bin/${entry.name}";
                text = ''
                  #!${pkgs.runtimeShell}
                  exec ${xBin} ${lib.escapeShellArg entry.name} "$@"
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
      resolveShortcutTarget = command:
        if command == "x" && cfg.menu.enable
        then lib.getExe' menuBin "x"
        else if entriesByName ? ${command}
        then let
          entry = entriesByName.${command};
          head = builtins.head (lib.splitString " " entry.run);
        in
          if needsWrapper entry
          then "${lib.getExe' menuBin "x"} ${lib.escapeShellArg entry.name}"
          else if entry.builtinSurface == "x" && cfg.menu.enable
          then lib.getExe' menuBin "x"
          else if entry.builtinSurface == "docs" && docsEnabled
          then lib.getExe docsBin
          else if entry.builtinSurface == "motd" && cfg.motd.enable
          then lib.getExe motdBin
          else lib.escapeShellArg head
        else if command == "menu" && cfg.menu.enable
        then lib.getExe' menuBin "x"
        else if command == "docs" && docsEnabled
        then lib.getExe docsBin
        else if command == "motd" && cfg.motd.enable
        then lib.getExe motdBin
        else lib.escapeShellArg command;
      shortcutWrappers =
        map (
          s:
            pkgs.writeTextFile {
              # Alias may be `?` or other non-store-safe glyphs; sanitize the
              # derivation name while keeping the bin/ entry exact.
              name = "prelude-shortcut-${lib.replaceStrings ["?" ":" "/" " "] ["q" "-" "-" "-"] s.alias}";
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
        paths =
          [
            menuBin
          ]
          ++ commandWrappers
          ++ shortcutWrappers
          ++ commandRuntimePackages
          ++ lib.optional cfg.menu.just.enable pkgs.just
          ++ lib.optional docsEnabled docsPkg;
        passthru = {
          inherit
            commandNames
            commandWrappers
            commandRuntimePackages
            shortcutAliases
            shortcutWrappers
            ;
          menuConfig = menuBin.configFile;
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
        if cfg.motd.enable || cfg.menu.enable
        then docsBin
        else
          pkgs.symlinkJoin {
            name = "docs";
            paths = [docsBin] ++ shortcutWrappers;
            passthru = {inherit shortcutAliases shortcutWrappers;};
            meta = {
              inherit (docsBin.meta) description;
              mainProgram = "docs";
            };
          };
      # Keep invalid local-server keys fail-closed even when a custom prompt
      # suppresses Prelude's generated status package.
      promptStatusPkg = assert builtins.deepSeq promptLocalServer true;
        if cfg.prompt.enable && cfg.prompt.configFile == null && promptLocalServer != null
        then
          mkPromptStatus deps (
            shared
            // {
              inherit
                (promptLocalServer)
                command
                check
                ttl
                start
                ;
            }
          )
        else null;
      # Resolve the palette and shell-only shadow once for every consumer.
      backdropPalette = plib.resolveBackdropPalette cfg.theme cfg.palette;
      pal = backdropPalette.palette;
      promptArtifacts = mkPrompt deps (
        generatorConfig cfg.prompt
        // {
          shortcuts = internalShortcuts;
          resolvedPalette = pal;
        }
      );
      promptPkg = promptArtifacts.live;
      promptFinalPkg = promptArtifacts.final;

      # `prelude` is the stable, namespaced CLI surface. Individual binaries
      # remain available for scripts, while the dispatcher prevents generic
      # names such as `setup` from entering consumers' PATH.
      preludeCli = pkgs.writeShellApplication {
        name = "prelude";
        runtimeInputs = [pkgs.coreutils];
        text = ''
          command="''${1:-help}"
          if [ "$#" -gt 0 ]; then
            shift
          fi

          case "$command" in
            help|-h|--help)
              cat <<'EOF'
          usage: prelude <command> [args...]

          Commands:
            hook           print the shell hook to add to your shell rc file
            setup          generate a Prelude project configuration
            title          choose and render a MOTD title
            title-previews render every bundled title font
          ${lib.optionalString cfg.motd.enable "  motd           render the welcome banner"}
          ${lib.optionalString cfg.menu.enable "  menu           open the command menu\n  x              dispatch a project command"}
          ${lib.optionalString docsEnabled "  docs            browse project documentation"}
          ${lib.optionalString cfg.portal.enable "  portal         launch an app, with live health lights\n  portal-web     the same launcher as a local web page"}
          EOF
              ;;
            hook)
              # Resolve the dialect here, in the user's shell, rather than at
              # build time: a Nix builder is always Bash, so a build-time guess
              # would hand Bash syntax to every consumer regardless of what they
              # actually run. $SHELL is the user's real shell and survives
              # environment capture untouched.
              shell="''${1:-}"
              if [ -z "$shell" ]; then
                shell="''${SHELL:-}"
                shell="''${shell##*/}"
              fi
              case "$shell" in
                bash) exec cat ${shellRuntime}/hook.bash ;;
                zsh) exec cat ${shellRuntime}/hook.zsh ;;
                *)
                  echo "prelude: hook: unsupported shell '$shell'" >&2
                  echo "hint: prelude hook [bash|zsh]" >&2
                  exit 2
                  ;;
              esac
              ;;
            setup)
              exec ${lib.getExe setupPkg} "$@"
              ;;
            title)
              exec ${lib.getExe titlePkg} "$@"
              ;;
            title-previews)
              exec ${lib.getExe titlePreviewsPkg} "$@"
              ;;
          ${lib.optionalString cfg.motd.enable ''
            motd)
              exec ${lib.getExe motdPkg} "$@"
              ;;
          ''}
          ${lib.optionalString cfg.menu.enable ''
            menu)
              exec ${lib.getExe menuPkg} "$@"
              ;;
            x)
              exec ${lib.getExe' menuPkg "x"} "$@"
              ;;
          ''}
          ${lib.optionalString docsEnabled ''
            docs)
              exec ${lib.getExe docsPkg} "$@"
              ;;
          ''}
          ${lib.optionalString cfg.portal.enable ''
            portal)
              exec ${lib.getExe' portalPkg "portal"} "$@"
              ;;
            portal-web)
              exec ${lib.getExe' portalPkg "portal-web"} "$@"
              ;;
          ''}
            *)
              echo "prelude: unknown command '$command'" >&2
              echo "hint: run 'prelude --help'" >&2
              exit 2
              ;;
          esac
        '';
        meta.description = "Prelude command-line interface";
      };

      # The current shell is the product boundary. Checked-in shell modules
      # own behavior; Nix injects paths and serializes the same normalized
      # catalogue used by menu. The devshell sources this entrypoint directly.
      shell =
        mkShellInit
        {
          inherit
            (pkgs)
            lib
            writeText
            runCommand
            starship
            blesh
            bash-completion
            stdenv
            ;
        }
        {
          palette = pal;
          inherit (backdropPalette) shadow;
          projectName = cfg.project;
          navigation = internalShortcuts;
          commandEntries = commandEntries;
          motdCommand =
            if cfg.motd.enable
            then lib.getExe motdPkg
            else null;
          statusEnabled = cfg.prompt.configFile == null;
          promptFinalConfig = promptFinalPkg;
          promptStatusCommand =
            if promptStatusPkg == null
            then null
            else lib.getExe promptStatusPkg;
          promptStatusConfig =
            if promptStatusPkg == null
            then null
            else promptStatusPkg.configFile;
        };
      shellInit = shell.init;
      shellRuntime = shell.runtime;

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
        pkgs.bash-completion
      ];
      promptStatusPackages = lib.optional (promptStatusPkg != null) promptStatusPkg;
      preludePkg = pkgs.symlinkJoin {
        name = "prelude";
        paths = [preludeCli] ++ preludeComponentPaths ++ promptRuntimePackages ++ promptStatusPackages;
        postBuild = lib.optionalString cfg.prompt.enable ''
          mkdir -p "$out/nix-support" "$out/share/prelude/shell"
          cp -f ${shellInit} "$out/share/prelude/init.bash"
          cp -R ${shellRuntime}/. "$out/share/prelude/shell/"
          # symlinkJoin may inherit a component hook as a read-only store
          # symlink. Replace it with Prelude's aggregate hook below.
          rm -f "$out/nix-support/setup-hook"
          cat > "$out/nix-support/setup-hook" <<'EOF'
          # This generated config remains the canonical serialized menu
          # catalogue and palette for tools that need the JSON boundary.
          export PRELUDE_MENU_CONFIG=${menuBin.configFile}

          # Exported as a plain variable, unlike the `prelude-init` function
          # below, because a variable is all an environment loader is
          # guaranteed to carry. lorri applies environment variables and never
          # runs shellHook (nix-community/lorri#159), so a shell function does
          # not survive into the user's shell at all. `prelude hook` sources
          # this path from the prompt, which is what makes activation work
          # identically under lorri, direnv, and `nix develop`.
          export PRELUDE_INIT=${shellInit}
          ${lib.optionalString cfg.prompt.enable ''
            # Export the generated starship config path from the setup-hook (not
            # shellHook) so direnv `use flake` picks it up — direnv re-emits
            # setup-hook exports on every reload and unloads them on exit, which
            # gives the auto-revert behavior the prompt promises. shellHook only
            # fires under `nix develop`, so a consumer who relied on it alone
            # would lose the themed prompt under direnv.
            export STARSHIP_CONFIG=${promptPkg}
          ''}

          # `prelude-init` mutates this shell, so it is a shell function rather
          # than an executable subprocess. The generated file is idempotent.
          prelude-init() {
            # shellcheck source=/dev/null
            . ${shellInit}
          }

          # setup-hooks run while Nix constructs the environment; the final
          # shellHook is what runs in the real interactive shell. Source the
          # init after the consumer hook so STARSHIP_CONFIG is already set.
          if [ -z "''${_prelude_init_registered:-}" ]; then
            _prelude_init_registered=1
            shellHook="''${shellHook-}
          . ${shellInit}"
          fi
          EOF
          chmod +x "$out/nix-support/setup-hook"
        '';
        passthru =
          {
            inherit
              preludeComponentPaths
              promptRuntimePackages
              promptStatusPkg
              promptStatusPackages
              shellInit
              shellRuntime
              ;
            menuConfig = menuBin.configFile;
          }
          // lib.optionalAttrs cfg.prompt.enable {
            prompt = promptPkg;
          };
        meta = {
          description = "Prelude devshell UI, command dispatcher, and enabled runtime dependencies";
          mainProgram = "prelude";
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
          # Prelude component, the namespaced CLI, and its runtime dependencies.
          packages.prelude = preludePkg;
          apps.prelude = mkApp preludePkg;
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
        (lib.mkIf cfg.portal.enable {
          packages.prelude-portal = portalPkg;
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
