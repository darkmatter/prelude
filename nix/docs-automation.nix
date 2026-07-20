# Deterministic documentation inputs plus an impure VHS recording app.
# Markdown and media fingerprints are checked in pure derivations; GIF/PNG
# bytes are produced outside the Nix sandbox because VHS needs a browser/TTY.
{ pkgs
, lib
, config
, ...
}:
let
  ex = import ../src/prelude/examples.nix;

  motdDemos = import ./motd-demo-builder.nix {
    inherit pkgs lib;
    currentMotdConfig = config.packages.motd.motdRenderConfig;
  };
  menuDemo = import ./menu-demo-builder.nix { inherit pkgs lib; };

  motdProgram = lib.getExe motdDemos.examplePackages.example-motd;
  menuProgram = lib.getExe menuDemo.package;
  minimalProgram = lib.getExe motdDemos.examplePackages.example-minimal;
  surfaceProgram = lib.getExe motdDemos.examplePackages.example-surface;

  vhsVisualSettings = ''
    Set FontFamily "MonaspiceNe Nerd Font Mono"
    Set FontSize 14
    Set LineHeight 1.0
    Set LetterSpacing 0
    Set Theme {"name":"prelude-minted","black":"#0c0c13","red":"#ee848e","green":"#b7ce99","yellow":"#f2c17d","blue":"#89b4fa","magenta":"#CC99FF","cyan":"#89b4fa","white":"#b1b1bf","brightBlack":"#4a5585","brightRed":"#ee848e","brightGreen":"#b7ce99","brightYellow":"#f2c17d","brightBlue":"#89b4fa","brightMagenta":"#CC99FF","brightCyan":"#89b4fa","brightWhite":"#b1b1bf","background":"#0c0c13","foreground":"#b1b1bf","selection":"#f2cdcd","cursor":"#f2cdcd"}
  '';

  motdTapeText = ''
    Output docs/media/motd.gif

    Set Shell "bash"
    ${vhsVisualSettings}
    Set Width 1100
    Set Height 940
    Set Padding 12
    Set Framerate 30
    Set TypingSpeed 50ms

    Hide
    Type "export PS1=; clear"
    Enter
    Type "tput civis; clear; ./docs/.record-bin/example-motd"
    Enter
    Sleep 400ms
    Show
    Sleep 4s
  '';

  menuTapeText = ''
    Output docs/media/menu.gif

    Set Shell "bash"
    ${vhsVisualSettings}
    Set Width 1100
    Set Height 780
    Set Padding 12
    Set Framerate 30
    Set TypingSpeed 80ms

    Hide
    Type "export PS1=; clear"
    Enter
    Show

    Type "./docs/.record-bin/example-menu"
    Enter
    Sleep 2s
    Type "dev"
    Sleep 1500ms
    Tab
    Sleep 2500ms
    Enter
    Sleep 2s
    Tab
    Sleep 1s
    Enter
    Sleep 1500ms
    Type " --host 0.0.0.0"
    Sleep 3s
  '';

  mkStillTapeText = name: width: height: ''
    Output docs/media/.${name}.gif

    Set Shell "bash"
    ${vhsVisualSettings}
    Set Width ${toString width}
    Set Height ${toString height}
    Set Padding 12
    Set Framerate 20

    Hide
    Type "export PS1=; clear"
    Enter
    Type "tput civis; clear; ./docs/.record-bin/example-${name}"
    Enter
    Sleep 400ms
    Show
    Sleep 2s
  '';
  minimalTapeText = mkStillTapeText "minimal" 1000 440;
  surfaceTapeText = mkStillTapeText "surface" 1100 560;

  motdTape = pkgs.writeText "prelude-motd.tape" motdTapeText;
  menuTape = pkgs.writeText "prelude-menu.tape" menuTapeText;
  minimalTape = pkgs.writeText "prelude-minimal.tape" minimalTapeText;
  surfaceTape = pkgs.writeText "prelude-surface.tape" surfaceTapeText;

  # Keep fingerprints independent of host-specific store paths. Reading file
  # names and contents also avoids making the manifest depend on the dirty
  # flake source path (and therefore indirectly on the manifest itself).
  readTree =
    path:
    let
      entries = builtins.readDir path;
      names = lib.sort builtins.lessThan (builtins.attrNames entries);
    in
    lib.concatMapStringsSep "\n"
      (
        name:
        let
          kind = entries.${name};
          child = path + "/${name}";
        in
        if kind == "directory" then
          "directory:${name}\n${readTree child}"
        else if kind == "regular" then
          "file:${name}\n${builtins.readFile child}"
        else
          "${kind}:${name}"
      )
      names;

  sharedInput = builtins.concatStringsSep "\n" [
    (builtins.readFile ../flake.lock)
    (builtins.readFile ../src/go.mod)
    (builtins.readFile ../src/go.sum)
    (builtins.readFile ../src/prelude/defaults.nix)
    (builtins.readFile ../src/prelude/lib.nix)
    (builtins.readFile ../src/prelude/themes.nix)
    (readTree ../src/pkg/shared)
    pkgs.vhs.version
  ];
  fingerprint =
    componentInput: tapeText: config:
    builtins.hashString "sha256" (
      builtins.concatStringsSep "\n" [
        sharedInput
        componentInput
        tapeText
        (builtins.toJSON config)
      ]
    );

  motdComponentInput = builtins.concatStringsSep "\n" [
    (builtins.readFile ./motd-demo-builder.nix)
    (builtins.readFile ../src/prelude/motd.nix)
    (readTree ../src/internal/motd)
  ];
  motdFingerprint = fingerprint motdComponentInput motdTapeText ex.motd;
  minimalFingerprint = fingerprint motdComponentInput minimalTapeText ex.motdDemos.minimal;
  surfaceFingerprint = fingerprint motdComponentInput surfaceTapeText ex.motdDemos.surface;
  menuFingerprint = fingerprint
    (builtins.concatStringsSep "\n" [
      (builtins.readFile ./menu-demo-builder.nix)
      (builtins.readFile ../src/prelude/menu.nix)
      (readTree ../src/internal/menu)
    ])
    menuTapeText
    ex.menu;

  manifestData = {
    version = 1;
    recordings = {
      motd = {
        fingerprint = motdFingerprint;
        outputs = [
          "motd.gif"
          "motd.png"
        ];
      };
      menu = {
        fingerprint = menuFingerprint;
        outputs = [
          "menu.gif"
          "menu.png"
        ];
      };
      minimal = {
        fingerprint = minimalFingerprint;
        outputs = [ "minimal.png" ];
      };
      surface = {
        fingerprint = surfaceFingerprint;
        outputs = [ "surface.png" ];
      };
    };
  };
  manifest = pkgs.writeText "prelude-doc-media-manifest.json" (builtins.toJSON manifestData + "\n");

  motdModuleConfig =
    config:
    let
      sharedNames = [
        "theme"
        "palette"
        "colorProfile"
        "project"
      ];
    in
    (lib.filterAttrs (name: _value: builtins.elem name sharedNames) config)
    // lib.optionalAttrs ((config.commandCatalog or { }) != { }) {
      commands = config.commandCatalog;
    }
    // {
      motd = builtins.removeAttrs config (sharedNames ++ [ "commandCatalog" ]);
    };

  gallery = pkgs.writeText "prelude-showcases.md" ''
    # Terminal showcases

    > Generated by `nix run .#docs-sync`. Do not edit this file directly.

    ## Welcome banner

    The MOTD composes project identity, static status, environment versions,
    next-step commands, and recipes. Navigation shortcuts appear automatically
    for enabled Prelude components.

    ![Prelude MOTD terminal recording](../media/motd.gif)

    A still image is available for renderers that do not animate GIFs:
    [motd.png](../media/motd.png).

    <details>
    <summary>Show the configuration used for this recording</summary>

    ```nix
    prelude = ${lib.generators.toPretty { } (motdModuleConfig ex.motd)};
    ```

    </details>

    ### Explicit description styling

    `prelude.motd.description.foreground` and `.italic` override the active
    theme for one description, while `prelude.motd.align = "left"` keeps the
    compact banner anchored to the terminal edge.

    ![Minimal MOTD with explicit description styling](../media/minimal.png)

    ```nix
    prelude = ${lib.generators.toPretty { } (motdModuleConfig ex.motdDemos.minimal)};
    ```

    ### Full-window background

    With `prelude.motd.clearScreen = true`, `windowBackground = true` paints
    the entire cleared terminal with the theme background. Without clearing,
    it fills the gutters and line remainders of emitted rows. Static keyed
    statuses appear in the header without running environment probes.

    ![MOTD with a full-window background](../media/surface.png)

    ```nix
    prelude = ${lib.generators.toPretty { } (motdModuleConfig ex.motdDemos.surface)};
    ```

    ## Interactive command menu

    The menu demonstrates live filtering, command details, argument suggestion
    chips, required-value validation, and a command preview. The recording
    selects `dev`, opens its details, accepts the `--port 3000` chip, and types
    `--host 0.0.0.0`.

    ![Prelude interactive menu recording](../media/menu.gif)

    A still of the final argument-entry state is available at
    [menu.png](../media/menu.png).

    <details>
    <summary>Show the configuration used for this recording</summary>

    ```nix
    prelude = ${
      lib.generators.toPretty { } {
        project = ex.menu.project;
        commands = ex.menu.commands;
      }
    };
    ```

    </details>
  '';

  optionModules = [
    ../src/prelude/options/shared.nix
    ../src/prelude/options/motd.nix
    ../src/prelude/options/menu.nix
    ../src/prelude/options/docs.nix
    ../src/prelude/options/prompt.nix
  ];
  evaluatedOptions = lib.evalModules { modules = optionModules; };
  validatedMotdConfigs =
    map
      (
        config:
        (lib.evalModules {
          modules = optionModules ++ [{ prelude = motdModuleConfig config; }];
        }).config.prelude.motd
      )
      [
        ex.motd
        ex.motdDemos.minimal
        ex.motdDemos.surface
      ];
  optionsDoc = pkgs.nixosOptionsDoc {
    options = {
      inherit (evaluatedOptions.options) prelude;
    };
    # Store-path declarations make generated Markdown change whenever the dirty
    # flake source path changes. The option names already identify their source.
    transformOptions = option: option // { declarations = [ ]; };
  };
  optionsReference = pkgs.runCommand "prelude-options-reference.md" { } ''
    {
      echo '# Options reference'
      echo
      echo '> Generated by `nix run .#docs-sync`. Do not edit this file directly.'
      echo
      cat ${optionsDoc.optionsCommonMark}
    } | awk '
      NF { for (i = 0; i < blank; i++) print ""; print; blank = 0; next }
      { blank++ }
    ' > "$out"
  '';

  sync = pkgs.writeShellApplication {
    name = "docs-sync";
    runtimeInputs = [
      pkgs.git
      pkgs.coreutils
    ];
    text = ''
      root=$(git rev-parse --show-toplevel)
      mkdir -p "$root/docs/generated" "$root/docs/reference"
      install -m 0644 ${gallery} "$root/docs/generated/showcases.md"
      install -m 0644 ${optionsReference} "$root/docs/reference/options.md"
      echo "updated docs/generated/showcases.md"
      echo "updated docs/reference/options.md"
    '';
  };

  record = pkgs.writeShellApplication {
    name = "docs-record";
    runtimeInputs = [
      pkgs.coreutils
      pkgs.ffmpeg
      pkgs.gifsicle
      pkgs.git
      pkgs.jq
      pkgs.ncurses
      pkgs.vhs
    ];
    text = ''
      root=$(git rev-parse --show-toplevel)
      cd "$root"
      mkdir -p docs/media docs/.record-bin docs/generated docs/reference
      trap 'rm -rf docs/.record-bin' EXIT
      ln -s ${motdProgram} docs/.record-bin/example-motd
      ln -s ${menuProgram} docs/.record-bin/example-menu
      ln -s ${minimalProgram} docs/.record-bin/example-minimal
      ln -s ${surfaceProgram} docs/.record-bin/example-surface

      current_fingerprint() {
        local name=$1
        if [ -f docs/media/manifest.json ]; then
          jq -r --arg name "$name" '.recordings[$name].fingerprint // ""' \
            docs/media/manifest.json 2>/dev/null || true
        fi
      }

      record_still() {
        local name=$1
        local expected=$2
        local tape=$3
        local current
        current=$(current_fingerprint "$name")
        if [ "$current" != "$expected" ] || [ ! -s "docs/media/$name.png" ]; then
          echo "recording $name option showcase"
          vhs --quiet "$tape"
          ffmpeg -y -v error -sseof -0.5 -i "docs/media/.$name.gif" \
            -frames:v 1 "docs/media/$name.png"
          rm -f "docs/media/.$name.gif"
        else
          echo "$name option showcase is current"
        fi
      }

      motd_current=$(current_fingerprint motd)
      if [ "$motd_current" != "${motdFingerprint}" ] \
        || [ ! -s docs/media/motd.gif ] \
        || [ ! -s docs/media/motd.png ]; then
        echo 'recording motd showcase'
        vhs --quiet ${motdTape}
        gifsicle -O3 --batch docs/media/motd.gif
        ffmpeg -y -v error -sseof -0.5 -i docs/media/motd.gif \
          -frames:v 1 docs/media/motd.png
      else
        echo 'motd showcase is current'
      fi

      record_still minimal ${minimalFingerprint} ${minimalTape}
      record_still surface ${surfaceFingerprint} ${surfaceTape}

      menu_current=$(current_fingerprint menu)
      if [ "$menu_current" != "${menuFingerprint}" ] \
        || [ ! -s docs/media/menu.gif ] \
        || [ ! -s docs/media/menu.png ]; then
        echo 'recording menu showcase'
        vhs --quiet ${menuTape}
        gifsicle -O3 --batch docs/media/menu.gif
        ffmpeg -y -v error -sseof -0.5 -i docs/media/menu.gif \
          -frames:v 1 docs/media/menu.png
      else
        echo 'menu showcase is current'
      fi

      install -m 0644 ${manifest} docs/media/manifest.json
      install -m 0644 ${gallery} docs/generated/showcases.md
      install -m 0644 ${optionsReference} docs/reference/options.md
      echo 'documentation media and generated Markdown are current'
    '';
  };

  docsFresh =
    assert builtins.deepSeq validatedMotdConfigs true;
    pkgs.runCommand "docs-generated-fresh" { } ''
      failed=0
      compare() {
        expected=$1
        actual=$2
        if [ ! -f "$actual" ] || ! cmp -s "$expected" "$actual"; then
          echo "stale generated documentation: $actual" >&2
          failed=1
        fi
      }
      compare ${gallery} ${../.}/docs/generated/showcases.md
      compare ${optionsReference} ${../.}/docs/reference/options.md
      if [ "$failed" -ne 0 ]; then
        echo 'run: nix run .#docs-sync' >&2
        exit 1
      fi
      touch "$out"
    '';

  mediaFresh = pkgs.runCommand "docs-media-fresh" { } ''
    failed=0
    if [ ! -f ${../.}/docs/media/manifest.json ] \
      || ! cmp -s ${manifest} ${../.}/docs/media/manifest.json; then
      echo 'documentation media fingerprints are stale' >&2
      failed=1
    fi
    for artifact in motd.gif motd.png menu.gif menu.png minimal.png surface.png; do
      if [ ! -s "${../.}/docs/media/$artifact" ]; then
        echo "missing documentation media: docs/media/$artifact" >&2
        failed=1
      fi
    done
    if [ "$failed" -ne 0 ]; then
      echo 'run: nix run .#docs-record' >&2
      exit 1
    fi
    touch "$out"
  '';
in
{
  inherit
    docsFresh
    gallery
    manifest
    mediaFresh
    menuTape
    motdTape
    optionsReference
    record
    sync
    ;
}
