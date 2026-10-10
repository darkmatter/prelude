# Public consumer contracts: both entrypoints author the same enabled packages;
# only configured launchers, never native compilation, depend on consumer data.
{
  pkgs,
  lib,
  flakePartsLib,
  localFlake,
}: let
  system = pkgs.stdenv.hostPlatform.system;
  drvPaths = lib.mapAttrs (_: package: package.drvPath);
  evaluate = fixturePkgs: configuration: {
    viaFunction = (localFlake.lib.evalModule fixturePkgs configuration).packages;
    viaModule =
      (flakePartsLib.evalFlakeModule {inputs.self.outPath = toString configuration.prelude.root;} {
        systems = [system];
        imports = [localFlake.flakeModules.default configuration];
        perSystem = {...}: {
          _module.args.pkgs = fixturePkgs;
        };
      }).config.allSystems.${
        system
      }.packages;
  };
  mkFixture = name: theme: promptEnabled: let
    root = pkgs.runCommand "workspace-consumer-${name}-root" {} ''
      mkdir -p "$out"
      printf '${name}-published:\n  echo ${name} published recipe\n' > "$out/justfile"
    '';
    configuration.prelude = {
      project = "workspace-${name}";
      inherit root theme;
      workspace.enable = true;
      menu = {
        enable = true;
        just.enable = true;
      };
      motd = {
        enable = true;
        clearScreen = false;
        title.style = "plain";
        header.tagline.text = "${name} MOTD sentinel";
        description.text = "${name} consumer welcome";
      };
      prompt.enable = promptEnabled;
      commands = {
        "${name}-task" = {
          description = "${name} catalogue sentinel";
          exec = "echo ${name} task";
          motd = 1;
        };
        where = {
          description = "show the command Root";
          exec = ''pwd -P; printf 'root=%s\n' "''${PRELUDE_ROOT-}"'';
        };
      };
      docs.pages = [{text = pkgs.writeText "workspace-${name}.md" "# ${name} guide\n\n${name} documentation sentinel\n";}];
    };
    evaluated = evaluate pkgs configuration;
  in
    evaluated
    // {
      inherit name root configuration;
      packages = evaluated.viaFunction;
      workspace = evaluated.viaFunction.prelude-shell.workspace;
    };
  a = mkFixture "alpha" "minted" true;
  # Workspace prerequisites deliberately do not include standalone prompt activation.
  b = mkFixture "beta" "paper" false;
  fixtures = [a b];
  workspaceArtifacts = packages: let
    workspace = packages.prelude-shell.workspace;
  in {
    derivations = drvPaths (lib.getAttrs ["package" "native" "check" "shell"] workspace);
    promptConfig = toString workspace.promptConfig;
    inherit (workspace) promptInit;
  };
  matchesEntrypoints = fixture:
    lib.assertMsg (fixture.viaFunction ? prelude-workspace && fixture.viaModule ? prelude-workspace)
    "${fixture.name}: enabled consumers must publish prelude-workspace through both entrypoints"
    && lib.assertMsg (drvPaths fixture.viaFunction == drvPaths fixture.viaModule)
    "${fixture.name}: lib.evalModule and flakeModules.default disagree on consumer package derivations"
    && lib.assertMsg (workspaceArtifacts fixture.viaFunction == workspaceArtifacts fixture.viaModule)
    "${fixture.name}: lib.evalModule and flakeModules.default disagree on configured workspace artifacts";

  disabledConfigurations = [
    {
      prelude = {
        project = "workspace-disabled";
        root = a.root;
      };
    }
    {prelude = (removeAttrs a.configuration.prelude ["workspace"]) // {prompt.enable = false;};}
  ];
  disabled = map (evaluate pkgs) disabledConfigurations;
  poisonPkgs = pkgs // {libghostty-vt = throw "disabled workspace evaluated libghostty-vt";};
  disabledWithPoison = map (evaluate poisonPkgs) disabledConfigurations;
  isDisabled = packages:
    !(packages ? prelude-workspace) && !(packages.prelude-shell ? workspace);
  rejected = configuration: let
    evaluated = evaluate pkgs configuration;
    attempt = packages: builtins.tryEval (builtins.deepSeq (drvPaths packages) true);
  in
    !(attempt evaluated.viaFunction).success && !(attempt evaluated.viaModule).success;
  missingPrerequisites = map (override: lib.recursiveUpdate a.configuration override) [
    {prelude.menu.enable = false;}
    {prelude.motd.enable = false;}
    {prelude.docs.pages = [];}
  ];
  disabledClosures = map (evaluated:
    pkgs.closureInfo {
      rootPaths = [evaluated.viaFunction.prelude-shell evaluated.viaFunction.prelude];
    })
  disabled;
  # Discard context only for forbidden paths: checking absence must not itself
  # build native or the renderer. closureInfo still computes real closures.
  forbiddenPath = package: lib.escapeShellArg (builtins.unsafeDiscardStringContext (toString package));
  nativeAndRenderer =
    [a.packages.prelude-workspace.native]
    ++ map (output: pkgs.libghostty-vt.${output}) pkgs.libghostty-vt.outputs;
  disabledComponents = [
    a.packages.prelude-motd
    a.packages.prelude-motd.componentRoot.renderer
    a.packages.prelude-menu
    a.packages.prelude-menu.componentRoot.menuTui
    a.packages.prelude-docs
    a.packages.prelude-docs.componentRoot.viewer
    a.packages.prelude-prompt
    pkgs.starship
    pkgs.blesh
    pkgs.bash-completion
  ];

  # Replace only the native launch/control endpoint in the already configured
  # public artifact. Its original wrapping code, PATH, Menu, Docs, MOTD, and
  # generated completion remain real; no private workspace builder is imported.
  nativeProbe = pkgs.writeShellScriptBin "prelude-workspace" ''
    case "''${1-}" in
      "") printf '%s\n' "$STARSHIP_CONFIG" ;;
      --environment)
        printf '%s\n' "$STARSHIP_CONFIG" "$PRELUDE_MENU_CONFIG" "$PRELUDE_COMPLETION_INIT"
        ;;
      *) exec "$@" ;;
    esac
  '';
  probeLauncher = package: package.overrideAttrs (_: {paths = [nativeProbe];});
  customSource = pkgs.writeText "workspace-custom-starship.toml" "format = 'custom'\n";
  completionProbe = pkgs.writeText "workspace-consumer-completion.bash" ''
    set -eo pipefail
    . "$PRELUDE_COMPLETION_INIT"
    # Use Bash's registered completion interface, not a private helper name.
    read -r command flag callback route <<< "$(complete -p x)"
    test "$command" = complete && test "$flag" = -F && test "$route" = x
    COMP_WORDS=(x "")
    COMP_CWORD=1
    "$callback"
    printf '%s\n' "''${COMPREPLY[@]}"
  '';
  manifest = pkgs.writeText "workspace-consumer-fixtures.json" (builtins.toJSON (map (fixture: {
      inherit (fixture) name;
      root = toString fixture.root;
      project = fixture.configuration.prelude.project;
      publishedLauncher = lib.getExe (probeLauncher fixture.packages.prelude-workspace);
      activeLauncher = lib.getExe (probeLauncher fixture.workspace.package);
      menu = lib.getExe' fixture.packages.prelude-menu "x";
      docs = lib.getExe fixture.packages.prelude-docs;
      motd = lib.getExe fixture.packages.prelude-motd;
      menuConfig = toString fixture.packages.prelude-menu.menuConfig;
      activeMenuConfig = toString fixture.packages.prelude-shell.menuConfig;
      completionInit = toString fixture.packages.prelude-shell.completionInit;
      docsConfig = "${fixture.packages.prelude-docs.componentRoot.config}/config.json";
      motdConfig = toString fixture.packages.prelude-motd.componentRoot.configFile;
      promptConfig = toString fixture.packages.prelude-workspace.promptConfig;
    })
    fixtures));
  mkPromptWorkspace = override:
    (localFlake.lib.evalModule pkgs (lib.recursiveUpdate a.configuration {prelude = override;})).packages.prelude-shell.workspace;
  customFile = mkPromptWorkspace {prompt.configFile = customSource;};
  customFormat = mkPromptWorkspace {prompt.settings.format = "custom";};
  tweaked = mkPromptWorkspace {
    theme = "paper";
    palette.accent = "#123456";
    prompt.settings = {
      add_newline = false;
      directory.truncation_length = 3;
    };
  };
  normal = a.packages.prelude-prompt;
  variant = a.packages.prelude-workspace.promptConfig;
  wrapper = probeLauncher a.packages.prelude-workspace;
in {
  workspace-consumer-contract = assert lib.all matchesEntrypoints fixtures;
  assert lib.assertMsg (lib.all rejected missingPrerequisites)
  "enabled workspace must explicitly reject missing Menu, MOTD, or Docs through both public entrypoints";
  assert lib.assertMsg (!(b.packages ? prelude-prompt))
  "the basic workspace consumer must not enable the standalone prompt";
  assert lib.assertMsg (a.packages.prelude-workspace.native.drvPath == b.packages.prelude-workspace.native.drvPath)
  "consumer project, catalogue, docs, theme, and prompt activation must not change the native derivation";
  assert lib.all (fixture:
    lib.assertMsg (fixture.workspace.native.drvPath == fixture.packages.prelude-workspace.native.drvPath)
    "${fixture.name}: active and published launchers must share native"
    && lib.assertMsg (!(fixture.workspace.native.doCheck or true))
    "${fixture.name}: consumer native builds must not run the configured PTY suite"
    && lib.assertMsg (fixture.workspace.check.drvPath != fixture.workspace.native.drvPath)
    "${fixture.name}: the configured native/PTY check must be separate"
    && lib.assertMsg (toString fixture.workspace.promptConfig
      == toString fixture.packages.prelude-workspace.promptConfig
      && fixture.workspace.promptInit == fixture.packages.prelude-workspace.promptInit)
    "${fixture.name}: public launcher must expose the configured prompt artifacts")
  fixtures;
  assert lib.assertMsg (lib.all (artifact: artifact a != artifact b) [
    (fixture: fixture.packages.prelude-workspace.drvPath)
    (fixture: fixture.workspace.package.drvPath)
    (fixture: fixture.workspace.check.drvPath)
    (fixture: toString fixture.packages.prelude-workspace.promptConfig)
    (fixture: toString fixture.packages.prelude-menu.menuConfig)
    (fixture: toString fixture.packages.prelude-shell.menuConfig)
    (fixture: toString fixture.packages.prelude-shell.completionInit)
    (fixture: toString fixture.packages.prelude-docs.componentRoot.config)
    (fixture: toString fixture.packages.prelude-motd.componentRoot.configFile)
  ]) "consumer fixtures must differ in configured packages and Config paths, not only their names";
    pkgs.runCommand "workspace-consumer-contract" {} ''
      ${lib.concatMapStringsSep "\n" (fixture: ''
          test -x ${fixture.packages.prelude-workspace}/bin/prelude-workspace
          test -x ${fixture.packages.prelude-shell}/bin/prelude-workspace
          cmp ${fixture.packages.prelude-shell}/bin/prelude-workspace ${fixture.workspace.package}/bin/prelude-workspace
          # Supply only Bash on caller PATH; the public app must bring its own
          # real published workspace launcher, not the native launch-only probe.
          if ! PATH=${lib.makeBinPath [pkgs.bash]} ${lib.getExe fixture.packages.prelude} x prelude:workspace --help \
            > "$TMPDIR/${fixture.name}-app-workspace-help" 2>&1; then
            cat "$TMPDIR/${fixture.name}-app-workspace-help" >&2
            exit 1
          fi
          grep -F 'prelude-workspace: an isolated interactive Bash workspace' "$TMPDIR/${fixture.name}-app-workspace-help"
        '')
        fixtures}
      touch "$out"
    '';

  workspace-disabled-closures = assert lib.assertMsg (!(localFlake.lib.evalModule poisonPkgs (builtins.head disabledConfigurations)).config.prelude.workspace.enable)
  "workspace activation must default to false";
  assert lib.assertMsg (lib.all (evaluated:
    isDisabled evaluated.viaFunction
    && isDisabled evaluated.viaModule
    && builtins.deepSeq (drvPaths evaluated.viaFunction) true
    && builtins.deepSeq (drvPaths evaluated.viaModule) true
    && drvPaths evaluated.viaFunction == drvPaths evaluated.viaModule)
  disabledWithPoison) "disabled consumers must stay lazy with poisoned libghostty-vt through both entrypoints";
    pkgs.runCommand "workspace-disabled-closures" {} ''
      for closure in ${lib.concatMapStringsSep " " toString disabledClosures}; do
        for forbidden in ${lib.concatMapStringsSep " " forbiddenPath nativeAndRenderer}; do
          if grep -Fxq "$forbidden" "$closure/store-paths"; then
            echo "$forbidden leaked into a disabled workspace closure" >&2
            exit 1
          fi
        done
        if grep -E -- '-(prelude-workspace|libghostty-vt)(-|$)' "$closure/store-paths"; then
          echo 'a native workspace/renderer variant leaked into a disabled consumer' >&2
          exit 1
        fi
      done
      for forbidden in ${lib.concatMapStringsSep " " forbiddenPath disabledComponents}; do
        if grep -Fxq "$forbidden" ${builtins.head disabledClosures}/store-paths; then
          echo "$forbidden leaked into a consumer with every component disabled" >&2
          exit 1
        fi
      done
      ${lib.concatMapStringsSep "\n" (evaluated: ''
          test ! -e ${evaluated.viaFunction.prelude-shell}/bin/prelude-workspace
        '')
        disabled}
      touch "$out"
    '';

  workspace-consumer-wrappers = pkgs.runCommand "workspace-consumer-wrappers" {nativeBuildInputs = [pkgs.python3];} ''
    export HOME="$TMPDIR/home" XDG_CACHE_HOME="$TMPDIR/cache" NO_COLOR=1
    mkdir -p "$HOME" "$XDG_CACHE_HOME"
    python3 - <<'PY'
    import json
    import os
    import re
    import subprocess
    import tomllib
    from pathlib import Path

    fixtures = json.loads(Path("${manifest}").read_text())
    checkout = Path(os.environ["TMPDIR"]) / "checkout"
    checkout.mkdir()
    (checkout / "justfile").write_text("checkout-only:\n  echo checkout recipe\n")

    def load(path):
        return json.loads(Path(path).read_text())

    def run(command, env):
        result = subprocess.run(command, cwd=checkout, env=env, text=True,
                                capture_output=True, timeout=20)
        assert result.returncode == 0, (command, result.returncode, result.stdout, result.stderr)
        return re.sub(r"\x1b\[[0-9;]*m", "", result.stdout)

    for fixture, other in ((fixtures[0], fixtures[1]), (fixtures[1], fixtures[0])):
        name, foreign = fixture["name"], other["name"]
        env = dict(os.environ, PRELUDE_MENU_CONFIG=other["menuConfig"],
                   PRELUDE_COMPLETION_INIT=other["completionInit"], STARSHIP_CONFIG="${customSource}")
        env.pop("PRELUDE_ROOT", None)
        env.pop("PRELUDE_ROOT_MENU", None)
        menu, active = load(fixture["menuConfig"]), load(fixture["activeMenuConfig"])
        docs, motd = load(fixture["docsConfig"]), load(fixture["motdConfig"])
        prompt = tomllib.loads(Path(fixture["promptConfig"]).read_text())
        assert menu["root"] == fixture["root"]
        assert active.get("root") is None
        for config in (menu, active, docs, motd):
            assert config["project"] == fixture["project"], config
            assert config["palette"] == menu["palette"], config
        assert prompt["palettes"]["prelude"]["bg"] == menu["palette"]["bg"]
        assert menu["palette"]["bg"] != load(other["menuConfig"])["palette"]["bg"]

        # These are the public, unmodified surface wrappers, not probes.
        listed = run([fixture["menu"], "--list"], env)
        assert f"{name}-task" in listed and f"{name} catalogue sentinel" in listed, listed
        assert f"{name}-published" in listed and "checkout-only" not in listed, listed
        assert f"{name} documentation sentinel" in run([fixture["docs"], "1"], env)
        rendered = run([fixture["motd"], "--pure"], env)
        assert f"{name} MOTD sentinel" in rendered and fixture["project"] in rendered.lower(), rendered

        # The native launch-only probe observes the real configured launcher:
        # stale inherited Configs must not select the other consumer's surfaces.
        for mode in ("published", "active"):
            launcher = fixture[f"{mode}Launcher"]
            expected_menu = fixture["menuConfig" if mode == "published" else "activeMenuConfig"]
            snapshot = run([launcher, "--environment"], env).splitlines()
            assert snapshot == ["${customSource}", expected_menu, fixture["completionInit"]], snapshot
            default_env = dict(env)
            default_env.pop("STARSHIP_CONFIG")
            snapshot = run([launcher, "--environment"], default_env).splitlines()
            assert snapshot == [fixture["promptConfig"], expected_menu, fixture["completionInit"]], snapshot
            listed = run([launcher, "x", "--list"], env)
            assert f"{name}-task" in listed and f"{foreign}-task" not in listed, listed
            imported = f"{name}-published" if mode == "published" else "checkout-only"
            absent = "checkout-only" if mode == "published" else f"{name}-published"
            assert imported in listed and absent not in listed, listed
            root = fixture["root"] if mode == "published" else str(checkout)
            inherited_root = fixture["root"] if mode == "published" else ""
            where = run([launcher, "x", "where"], env).splitlines()
            assert root in where and f"root={inherited_root}" in where, (mode, where)
            rendered_docs = run([launcher, "docs", "1"], env)
            assert f"{name} documentation sentinel" in rendered_docs, rendered_docs
            assert f"{foreign} documentation sentinel" not in rendered_docs, rendered_docs
            rendered_motd = run([launcher, "motd", "--pure"], env)
            assert f"{name} MOTD sentinel" in rendered_motd, rendered_motd
            assert f"{foreign} MOTD sentinel" not in rendered_motd, rendered_motd
            candidates = run([launcher, "bash", "--noprofile", "--norc", "${completionProbe}"], env).splitlines()
            assert f"{name}-task" in candidates and f"{foreign}-task" not in candidates, candidates
            assert imported in candidates and absent not in candidates, candidates

        overridden = run([fixture["publishedLauncher"], "x", "--list"], dict(env, PRELUDE_ROOT=str(checkout)))
        assert "checkout-only" in overridden and f"{name}-published" not in overridden, overridden
    PY
    touch "$out"
  '';

  workspace-prompt = assert toString (a.packages.prelude-shell.promptPreset {}).live == toString normal;
  assert customFile.package.promptConfig == customSource;
    pkgs.runCommand "workspace-prompt" {nativeBuildInputs = [pkgs.python3 pkgs.starship];} ''
      set -euo pipefail
      python3 - <<'PY'
      import tomllib
      from pathlib import Path

      def load(path):
          return tomllib.loads(Path(path).read_text())

      normal = load("${normal}")
      workspace = load("${variant}")
      keymap = (
          r"Alt + \[[m](bold fg:accent)\][─](fg:surface)motd"
          r"[──](fg:surface)\[[x](bold fg:accent)\][─](fg:surface)menu"
          r"[──](fg:surface)\[[d](bold fg:accent)\][─](fg:surface)docs"
      )
      normal_keymap = (
          r"\[[?](bold fg:accent)\][─](fg:surface)motd"
          r"[──](fg:surface)\[[x](bold fg:accent)\][─](fg:surface)menu"
          r"[──](fg:surface)\[[d](bold fg:accent)\][─](fg:surface)docs"
      )
      assert f"[{keymap}](fg:muted)" in workspace["format"]
      assert workspace["format"].count("Alt +") == 1
      assert "Ctrl+P" not in workspace["format"]
      assert f"[{normal_keymap}](fg:muted)" in normal["format"]
      assert "Alt +" not in normal["format"]
      normal.pop("format")
      workspace.pop("format")
      assert workspace == normal, "The preset must change only the keymap"
      assert load("${customFormat.package.promptConfig}")["format"] == "custom"
      tweaked = load("${tweaked.package.promptConfig}")
      assert tweaked["add_newline"] is False
      assert tweaked["directory"]["truncation_length"] == 3
      assert tweaked["palettes"]["prelude"]["accent"] == "#123456"
      assert tweaked["palettes"]["prelude"]["bg"] != normal["palettes"]["prelude"]["bg"]
      PY

      export HOME="$TMPDIR/home" XDG_CACHE_HOME="$TMPDIR/cache" NO_COLOR=1
      unset STARSHIP_SHELL
      mkdir -p "$HOME" "$XDG_CACHE_HOME"
      export STARSHIP_CONFIG=${variant}
      starship prompt --terminal-width 120 --status 0 > "$TMPDIR/prompt"
      python3 - "$TMPDIR/prompt" <<'PY'
      import re
      import sys
      from pathlib import Path

      rendered = Path(sys.argv[1]).read_text()
      plain = re.sub(r"\x1b\[[0-9;]*m", "", rendered)
      assert "Alt + [m]─motd──[x]─menu──[d]─docs" in plain.splitlines()[1], repr(plain)
      assert plain.count("Alt +") == 1
      assert "Ctrl+P" not in plain
      PY

      env -u STARSHIP_CONFIG ${lib.getExe wrapper} > "$TMPDIR/wrapper-default"
      STARSHIP_CONFIG= ${lib.getExe wrapper} > "$TMPDIR/wrapper-empty"
      STARSHIP_CONFIG=${normal} ${lib.getExe wrapper} > "$TMPDIR/wrapper-inherited"
      STARSHIP_CONFIG=${variant} ${lib.getExe wrapper} > "$TMPDIR/wrapper-variant"
      STARSHIP_CONFIG=${customSource} ${lib.getExe wrapper} > "$TMPDIR/wrapper-custom"
      python3 - "$TMPDIR" <<'PY'
      import sys
      from pathlib import Path

      root = Path(sys.argv[1])
      for name in ("default", "empty", "inherited", "variant"):
          assert (root / f"wrapper-{name}").read_text().splitlines() == ["${variant}"]
      assert (root / "wrapper-custom").read_text().splitlines() == ["${customSource}"]
      PY
      (
        export STARSHIP_CONFIG=${normal}
        ${a.workspace.shell.shellHook}
        test "$STARSHIP_CONFIG" = ${variant}
      )
      (
        export STARSHIP_CONFIG=${customSource}
        ${a.workspace.package.promptInit}
        test "$STARSHIP_CONFIG" = ${customSource}
      )
      (
        unset STARSHIP_CONFIG
        ${customFile.package.promptInit}
        test "$STARSHIP_CONFIG" = ${customSource}
      )
      (
        unset STARSHIP_CONFIG
        ${customFormat.shell.shellHook}
        test "$STARSHIP_CONFIG" = ${customFormat.package.promptConfig}
      )
      touch "$out"
    '';
}
