# TypeScript API (ts/): the shared library it loads, the Nix-owned data it is
# generated from, and the checks that keep it from drifting.
#
#   libprelude  Go menu/MOTD/docs as a C shared library (-buildmode=c-shared),
#               loaded through bun:ffi
#   generated   ts/src/internal/generated.ts: themes and defaults from src/prelude
#   expected    ts/test/fixtures/conformance.expected.json: the JSON Nix itself
#               generates for the shared conformance fixtures, which the TS
#               tests compare their own output against
#   sync        `x ts:sync` copies generated + expected into the tree
#   fresh       fails when either copy is stale
#   test        bun test against the built library
#   example     examples/typescript as the single-file executable its README
#               builds (`nix run .#example-typescript`)
{
  pkgs,
  lib,
  ...
}: let
  d = import ../src/prelude/defaults.nix;
  themes = import ../src/prelude/themes.nix;
  ext = pkgs.stdenv.hostPlatform.extensions.sharedLibrary;

  libprelude = pkgs.buildGo126Module {
    pname = "libprelude";
    version = "0.1.0";
    src = import ../src/prelude/go-source.nix {inherit lib;};
    vendorHash = "sha256-BHrU5pKVDuGDq0ZHbHKcUBa5olzHzfgoJXzv2IGXY4U=";
    # c-shared needs cgo; the CLI binaries are cgo-free and never build this.
    env.CGO_ENABLED = "1";
    doCheck = false;
    buildPhase = ''
      runHook preBuild
      go build -buildmode=c-shared -ldflags "-s -w" -o libprelude${ext} ./cmd/libprelude
      runHook postBuild
    '';
    installPhase = ''
      runHook preInstall
      install -Dm0555 libprelude${ext} "$out/lib/libprelude${ext}"
      install -Dm0444 libprelude.h "$out/include/libprelude.h"
      runHook postInstall
    '';
    meta.description = "Prelude's menu, MOTD, and docs as a C shared library for bun:ffi";
  };
  libraryPath = "${libprelude}/lib/libprelude${ext}";

  # TypeScript apps inherit the layout defaults but not the ACME example
  # content that makes a zero-config devshell banner look real.
  motdDefaults =
    removeAttrs d.motd ["enable"]
    // {
      header =
        d.motd.header
        // {
          tagline = d.motd.header.tagline // {text = "";};
          statusHint = d.motd.header.statusHint // {links = [];};
          status = {};
        };
      description = d.motd.description // {text = "";};
      env = [];
    };
  generatedData = pkgs.writeText "prelude-ts-generated.json" (builtins.toJSON {
    inherit themes;
    defaults = {
      inherit (d) theme colorProfile;
      menu = removeAttrs d.menu ["width"];
      motd = motdDefaults;
    };
  });
  generated = pkgs.runCommand "generated.ts" {nativeBuildInputs = [pkgs.jq];} ''
    {
      echo '// Generated from src/prelude/themes.nix and src/prelude/defaults.nix.'
      echo '// Do not edit: run `x ts:sync` after changing either file.'
      echo
      echo "export const themes = $(jq .themes ${generatedData}) as const;"
      echo
      echo "export const defaults = $(jq .defaults ${generatedData}) as const;"
    } > "$out"
  '';

  # Generator configs only: the Go derivations are never built here.
  generators = {
    inherit (pkgs) lib writeShellApplication writeText symlinkJoin;
    buildGoModule = args: args;
  };
  mkMenu = import ../src/prelude/menu.nix generators;
  mkMotd = import ../src/prelude/motd.nix generators;
  fixtures = lib.importJSON ../ts/test/fixtures/conformance.json;
  motdFixture = cfg:
    mkMotd (cfg
      // {
        # The flake module's names for the catalogue inputs.
        commandCatalog = cfg.commands or {};
        commandGroupOrder = cfg.groupOrder or [];
      });
  configFiles =
    lib.mapAttrs' (name: cfg: lib.nameValuePair "menu_${name}" (mkMenu cfg).configFile) fixtures.menu
    // lib.mapAttrs' (name: cfg: lib.nameValuePair "motd_${name}" (motdFixture cfg).configFile) fixtures.motd;
  # jq object body for one surface: `name: $<surface>_<name>[0], …`.
  section = surface:
    lib.concatMapStringsSep ", " (name: "${name}: $" + "${surface}_${name}[0]") (lib.attrNames fixtures.${surface});
  expected = pkgs.runCommand "conformance.expected.json" {nativeBuildInputs = [pkgs.jq];} ''
    jq -n ${lib.concatStringsSep " " (lib.mapAttrsToList (name: file: "--slurpfile ${name} ${file}") configFiles)} \
      '{menu: {${section "menu"}}, motd: {${section "motd"}}}' > "$out"
  '';

  sync = pkgs.writeShellApplication {
    name = "ts-sync";
    runtimeInputs = [
      pkgs.git
      pkgs.coreutils
    ];
    text = ''
      root=$(git rev-parse --show-toplevel)
      install -m 0644 ${generated} "$root/ts/src/internal/generated.ts"
      install -m 0644 ${expected} "$root/ts/test/fixtures/conformance.expected.json"
      echo "updated ts/src/internal/generated.ts"
      echo "updated ts/test/fixtures/conformance.expected.json"
    '';
  };

  fresh = pkgs.runCommand "ts-generated-fresh" {} ''
    failed=0
    compare() {
      if [ ! -f "$2" ] || ! cmp -s "$1" "$2"; then
        echo "stale generated TypeScript input: ''${2#${../ts}/}" >&2
        failed=1
      fi
    }
    compare ${generated} ${../ts}/src/internal/generated.ts
    compare ${expected} ${../ts}/test/fixtures/conformance.expected.json
    if [ "$failed" -ne 0 ]; then
      echo 'run: x ts:sync' >&2
      exit 1
    fi
    touch "$out"
  '';

  # bun test needs no node_modules: bun:test and bun:ffi are built in, and the
  # tests import the package by its own name.
  test =
    pkgs.runCommand "ts-api" {
      nativeBuildInputs = [pkgs.bun];
      PRELUDE_LIB = libraryPath;
    } ''
      cp -r ${../ts} ts
      chmod -R u+w ts
      cd ts
      export HOME="$TMPDIR"
      bun test
      touch "$out"
    '';

  # Bun, the app, its docs page, and libprelude in one `acme` binary. runCommand
  # skips stripping, which would cut off the bundle bun appends to its binary.
  example = let
    source = lib.fileset.toSource {
      root = ../.;
      fileset = lib.fileset.unions [
        ../examples/typescript
        ../ts/src
        ../ts/tsconfig.json
      ];
    };
  in
    pkgs.runCommand "example-typescript" {
      nativeBuildInputs = [pkgs.bun];
      meta.mainProgram = "acme";
    } ''
      cp -r ${source}/. .
      chmod -R u+w .
      cp ${libraryPath} examples/typescript/libprelude.so
      export HOME="$TMPDIR"
      mkdir -p "$out/bin"
      bun build --compile examples/typescript/standalone.ts --outfile "$out/bin/acme"
    '';
in {
  inherit
    example
    expected
    fresh
    generated
    libprelude
    libraryPath
    sync
    test
    ;
}
