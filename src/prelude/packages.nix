# Prelude's packages, built from its evaluated options. flakeModules.default
# (module.nix) and lib.evalModule (eval.nix) both build through here, so the
# two install paths produce the same derivations.
#
#   cfg       the evaluated `prelude` option set
#   commands  the whole command catalogue: `cfg.commands`, plus flake-parts'
#             per-system entries when that path supplies them
{
  lib,
  pkgs,
  cfg,
  commands,
}: let
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
  # The catalogue tree's top-level commands, including parents only their
  # subcommands declare (`db` for `"db migrate"`), and every node below them.
  commandNodes = plib.commandNodes commands;
  allCommandNodes = plib.flattenNodes commandNodes;
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
      ) "prelude.prompt.localServer.command must name a prelude.commands command by its words (\"db migrate\")";
        configured // {start = entry.xInvocation;};
  commandNames = map (entry: entry.name) commandEntries;
  selectedMotdCommands = plib.selectCommands allCommandNodes;
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
  # Wrappers belong to declared top-level commands, parents included, so a
  # declared `db` lets `db migrate` run bare. A root containing `:`
  # (`go:test`) and a parent only its subcommands declare (`go` for
  # `"go test"`) stay reachable through `x`, so neither shadows a real tool.
  wrappedCommandEntries = lib.filter (entry: entry.onPath && needsWrapper entry) commandNodes;
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
    "prelude: commands named \"menu\" or \"x\" cannot receive wrappers because Prelude owns those entrypoints";
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

  # What `nix run …#prelude-menu` and the `prelude` app open: the same menu,
  # bound to the project source, so it reads that project's files and runs
  # its commands there wherever it starts. The devshell keeps menuPkg, which
  # works in the caller's checkout.
  publishedMenuBin =
    if cfg.root == null
    then menuBin
    else
      mkMenu deps (menuRenderConfig
        // {
          inherit (cfg) root;
          runtimePackages = commandRuntimePackages ++ lib.optional cfg.menu.just.enable pkgs.just;
        });
  publishedMenuPkg =
    if cfg.root == null
    then menuPkg
    else
      pkgs.symlinkJoin {
        name = "menu";
        # menuPkg's command and shortcut wrappers call the unbound `x`, so
        # they stay out. The passthru still describes the catalogue they
        # come from, which the checks read.
        paths =
          [publishedMenuBin]
          ++ commandRuntimePackages
          ++ lib.optional cfg.menu.just.enable pkgs.just;
        passthru =
          menuPkg.passthru
          // {
            componentRoot = publishedMenuBin;
            menuConfig = publishedMenuBin.configFile;
          };
        inherit (menuPkg) meta;
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
  promptRenderConfig =
    generatorConfig cfg.prompt
    // {
      shortcuts = internalShortcuts;
      resolvedPalette = pal;
    };
  promptArtifacts = mkPrompt deps promptRenderConfig;
  # Keymap-only presets reuse the evaluated theme, settings, and configFile.
  promptPreset = {
    shortcuts ? internalShortcuts,
    prefix ? "",
  }:
    mkPrompt deps (promptRenderConfig
      // {
        inherit shortcuts;
        keymapPrefix = prefix;
      });
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
    menuSurface =
      if embedDependencies
      then publishedMenuPkg
      else menuPkg;
    # The self-contained app puts its own surfaces first on PATH, so a menu
    # entry such as `docs` or `portal` opens this project's, not the caller's.
    ownSurfaces =
      lib.optional cfg.menu.enable menuSurface
      ++ lib.optional cfg.motd.enable motdPkg
      ++ lib.optional docsEnabled docsPkg
      ++ lib.optional cfg.portal.enable portalPkg;
  in
    pkgs.writeShellApplication {
      name = "prelude";
      runtimeInputs = [pkgs.coreutils] ++ lib.optionals embedDependencies ownSurfaces;
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
          preflight      print activation code for a custom shellHook
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
            exec ${target menuSurface "menu"} "$@"
            ;;
          x)
            exec ${target menuSurface "x"} "$@"
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
      # Parents only their subcommands declare are shell commands too, so
      # completion and the status row see the whole tree.
      commandEntries = allCommandNodes;
      # `just <TAB>` uses just's own completion when recipes are imported.
      justImport = cfg.menu.just.enable;
      # `x <TAB>` offers the entries the menu imports at runtime, read
      # from the menu itself, so its candidates stay dispatchable.
      runtimeImports = cfg.menu.just.enable || cfg.menu.scripts.enable;
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
  completionInit = shell.completionInit;

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
    # preflight is unconditional so custom shellHooks can resolve it
    # regardless of which components are enabled.
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
      cp -f ${completionInit} "$out/share/prelude/completion-init.bash"
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
      export PRELUDE_COMPLETION_INIT=${completionInit}
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
          completionInit
          ;
        menuConfig = menuBin.configFile;
      }
      // lib.optionalAttrs cfg.prompt.enable {
        prompt = promptPkg;
        inherit promptPreset;
      };
    meta = {
      description = "Prelude shell runtime, PATH dispatcher, and activation";
      mainProgram = "prelude";
    };
  };
in
  # `prelude` backs the app/default-package surface; `prelude-shell` is the
  # closure-minimal devshell package.
  {
    prelude = preludeAppPkg;
    prelude-shell = preludeShellPkg;
    prelude-preflight = preflightPkg;
  }
  // lib.optionalAttrs cfg.motd.enable {
    prelude-motd = motdPkg;
    prelude-title = titlePkg;
    prelude-title-previews = titlePreviewsPkg;
    prelude-wizard = wizardPkg;
  }
  // lib.optionalAttrs cfg.menu.enable {
    prelude-menu = publishedMenuPkg;
  }
  // lib.optionalAttrs cfg.portal.enable {
    prelude-portal = portalPkg;
  }
  // lib.optionalAttrs docsEnabled {
    prelude-docs = docsPkg;
  }
  // lib.optionalAttrs cfg.prompt.enable {
    prelude-prompt = promptPkg;
  }
