# flake-parts module: the prelude devshell UI suite.
#
#   prelude.motd    — devshell welcome banner
#   prelude.menu    — interactive command menu
#   prelude.docs    — Markdown project docs viewer
#   prelude.prompt  — themed starship config (packages.prelude-prompt = starship.toml)
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
#           packages = [ config.packages.prelude-shell ];
#         };
#       };
#     };
#
# The module exports one self-contained app, `apps.prelude`, plus a
# `packages.prelude-shell` that bundles every enabled component. Add only that
# one package to the consumer devshell; the setup-hook handles activation.
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
  mkPreflight = import ./preflight.nix;
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
      wizardPkg = pkgs.writeShellApplication {
        # A generic `wizard` executable must never enter consumers' shells.
        # The stable installed name is `prelude-wizard`; the public bootstrap
        # interface is `nix run github:darkmatter/prelude -- wizard`.
        name = "prelude-wizard";
        runtimeInputs = [titlePkg];
        text = ''
          if [ "''${1:-}" = "--help" ] || [ "''${1:-}" = "-h" ]; then
            cat <<'EOF'
          usage: prelude wizard [--recipe path] [-o path]

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

      # Path-free by construction: the snippet delegates project-specific MOTD
      # behavior to `shellInit`.
      preflightPkg = mkPreflight {inherit (pkgs) writeShellApplication;} {
        inherit shellRuntime;
      };

      motdPkg = pkgs.symlinkJoin {
        name = "motd";
        # A component output exposes that component only. Consumers compose the
        # enabled MOTD, menu, and docs packages explicitly in their devshell;
        # the wizard and preview generators remain opt-in package outputs.
        paths = [motdBin] ++ lib.optionals (!cfg.menu.enable) shortcutWrappers;
        passthru = {
          componentRoot = motdBin;
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

      menuRenderConfig =
        generatorConfig cfg.menu
        // {
          inherit commands;
          groupOrder = sortCfg.groups;
        };
      menuBin = mkMenu deps menuRenderConfig;

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
      # is runnable. Targets owned by the same package use absolute paths;
      # cross-component targets stay on PATH so one component does not retain
      # another component's closure.
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
          then lib.escapeShellArg "docs"
          else if entry.builtinSurface == "motd" && cfg.motd.enable
          then lib.escapeShellArg "motd"
          else lib.escapeShellArg head
        else if command == "menu" && cfg.menu.enable
        then lib.getExe' menuBin "x"
        else if command == "docs" && docsEnabled
        then lib.escapeShellArg "docs"
        else if command == "motd" && cfg.motd.enable
        then lib.escapeShellArg "motd"
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
          ++ lib.optional cfg.menu.just.enable pkgs.just;
        passthru = {
          componentRoot = menuBin;
          inherit
            commandNames
            commandWrappers
            commandRuntimePackages
            shortcutAliases
            shortcutWrappers
            menuRenderConfig
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
      docsBasePkg =
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
      docsPkg =
        docsBasePkg
        // {
          componentRoot = docsBin;
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

      # Both surfaces share one dispatcher contract. The app embeds every
      # subcommand so `nix run <prelude-flake> -- ...` is self-contained; the
      # devshell dispatcher resolves component executables from PATH so adding
      # the shell core does not pull generators or disabled components into the
      # environment closure.
      mkPreludeCli = embedDependencies: let
        target = package: executable:
          if embedDependencies
          then lib.getExe' package executable
          else executable;
      in
        pkgs.writeShellApplication {
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
              preflight      print the shell code to eval from .envrc or shellHook
              wizard         generate a Prelude project configuration
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
              preflight)
                exec ${lib.getExe preflightPkg} "$@"
                ;;
              wizard)
                exec ${target wizardPkg "prelude-wizard"} "$@"
                ;;
              title)
                exec ${target titlePkg "prelude-title"} "$@"
                ;;
              title-previews)
                exec ${target titlePreviewsPkg "prelude-title-previews"} "$@"
                ;;
            ${lib.optionalString cfg.motd.enable ''
              motd)
                exec ${target motdPkg "motd"} "$@"
                ;;
            ''}
            ${lib.optionalString cfg.menu.enable ''
              menu)
                exec ${target menuPkg "menu"} "$@"
                ;;
              x)
                exec ${target menuPkg "x"} "$@"
                ;;
            ''}
            ${lib.optionalString docsEnabled ''
              docs)
                exec ${target docsPkg "docs"} "$@"
                ;;
            ''}
            ${lib.optionalString cfg.portal.enable ''
              portal)
                exec ${target portalPkg "portal"} "$@"
                ;;
              portal-web)
                exec ${target portalPkg "portal-web"} "$@"
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
      preludeShellCli = mkPreludeCli false;
      preludeAppPkg = mkPreludeCli true;

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
            then "motd"
            else null;
          # Build-time only: perturb PRELUDE_INIT when the MOTD rebuilds so the
          # prompt hook reloads it, without retaining the MOTD package or
          # exporting render state.
          motdRevision =
            if cfg.motd.enable
            then builtins.hashString "sha256" (builtins.unsafeDiscardStringContext (toString motdBin))
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
          promptEnabled = cfg.prompt.enable;
        };
      shellInit = shell.init;
      shellRuntime = shell.runtime;

      # Canonical shell-core package. Its dispatcher resolves components from
      # PATH, and the generated init invokes `motd` from PATH, so enabled
      # component packages are bundled into this closure. Consumers add only
      # this one package to their devshell.
      promptRuntimePackages = lib.optionals cfg.prompt.enable [
        pkgs.starship
        pkgs.blesh
        pkgs.bash-completion
      ];
      promptStatusPackages = lib.optional (promptStatusPkg != null) promptStatusPkg;
      preludeShellPkg = pkgs.symlinkJoin {
        name = "prelude-shell";
        # preflight is unconditional: it is the line consumers put in .envrc or a
        # shellHook, so it must resolve regardless of which components are on.
        paths =
          [
            preludeShellCli
            preflightPkg
          ]
          ++ promptRuntimePackages
          ++ promptStatusPackages
          # Enabled component packages are bundled so the consumer only adds
          # `config.packages.prelude-shell` to their devshell. The module's own
          # mkIf gates ensure only enabled components are included.
          ++ lib.optional cfg.motd.enable motdPkg
          ++ lib.optional cfg.menu.enable menuPkg
          ++ lib.optional docsEnabled docsPkg
          ++ lib.optional cfg.portal.enable portalPkg;
        # Always emitted. The MOTD is the module's core promise, and reaching it
        # from lorri requires PRELUDE_INIT to exist as an exported variable even
        # when the prompt component is off. With prompt disabled, `shellInit`
        # names no Starship/ble.sh/completion paths, so this costs those
        # consumers nothing in closure size.
        postBuild = ''
          mkdir -p "$out/nix-support" "$out/share/prelude/shell"
          cp -f ${shellInit} "$out/share/prelude/init.bash"
          cp -R ${shellRuntime}/. "$out/share/prelude/shell/"
          # The shell core owns exactly one setup hook.
          rm -f "$out/nix-support/setup-hook"
          cat > "$out/nix-support/setup-hook" <<'EOF'
          # This generated config remains the canonical serialized menu
          # catalogue and palette for tools that need the JSON boundary.
          export PRELUDE_MENU_CONFIG=${menuBin.configFile}

          # Exported as a plain variable, unlike the `prelude-init` function
          # below, because a variable is all an environment loader is
          # guaranteed to carry. lorri does run shellHook, but inside the Nix
          # builder — non-interactive, in the build directory
          # (nix-community/lorri#159) — so only the variables it exported reach
          # the user's shell; functions and terminal output do not. Sourcing
          # this path from an interactive shell is what actually renders the
          # MOTD under lorri, direnv, and `nix develop` alike.
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

          ${lib.optionalString cfg.prompt.enable ''
              # setup-hooks run while Nix constructs the environment; the final
              # shellHook is what runs in the real interactive shell. Source the
              # init after the consumer hook so STARSHIP_CONFIG is already set.
              #
              # Only appended when the prompt is enabled. MOTD-only projects are
              # documented to write `shellHook = "motd"` themselves, and appending
              # here as well would render the banner twice under `nix develop`.
              # Those projects reach the same init through `prelude hook` instead.
              # A consumer shellHook may already have evaluated preflight. The
              # init records that same-shell load without exporting it, so skip
              # only this automatic source; explicit `prelude-init` or preflight
              # calls remain deliberate MOTD reprints.
              if [ -z "''${_prelude_init_registered:-}" ]; then
                _prelude_init_registered=1
                shellHook="''${shellHook-}
            if [ \"\''${_PRELUDE_INIT_LOADED-}\" != ${lib.escapeShellArg (toString shellInit)} ]; then
              . ${shellInit}
            fi"
              fi
          ''}
          EOF
          chmod +x "$out/nix-support/setup-hook"
        '';
        passthru =
          {
            inherit
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
          description = "Prelude shell runtime, PATH dispatcher, and activation";
          mainProgram = "prelude";
        };
      };
    in
      lib.mkMerge [
        {
          # The module contributes one self-contained app. It intentionally uses
          # the embedded dispatcher rather than the PATH-resolving shell core.
          apps.prelude = {
            type = "app";
            program = lib.getExe preludeAppPkg;
          };

          # `packages.prelude` backs the app/default-package surface.
          # `packages.prelude-shell` is the closure-minimal devshell package.
          packages.prelude = preludeAppPkg;
          packages.prelude-shell = preludeShellPkg;
          packages.prelude-preflight = preflightPkg;
        }
        (lib.mkIf cfg.motd.enable {
          packages.prelude-motd = motdPkg;
          packages.prelude-title = titlePkg;
          packages.prelude-title-previews = titlePreviewsPkg;
          packages.prelude-wizard = wizardPkg;
        })
        (lib.mkIf cfg.menu.enable {
          packages.prelude-menu = menuPkg;
        })
        (lib.mkIf cfg.portal.enable {
          packages.prelude-portal = portalPkg;
        })
        (lib.mkIf docsEnabled {
          packages.prelude-docs = docsPkg;
        })
        (lib.mkIf cfg.prompt.enable {
          packages.prelude-prompt = promptPkg;
        })
      ];
  };
}
