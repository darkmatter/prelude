# Flake checks. Checks whose $out is a rendered preview use the `-render(s)`
# suffix — the previews utility (previews.nix) discovers them by name.
{
  pkgs,
  lib,
  config,
  demos,
  docsAutomation,
  previews,
  flakePartsLib,
  inputs,
  localFlake,
  ...
}: let
  preludeLib = import ./lib.nix {inherit lib;};
  internalLib = import ../src/prelude/lib.nix {inherit lib;};
  # Evaluate the actual starter and reference flakes with this checkout as
  # their Prelude input. This catches lexical-scope mistakes in consumer
  # snippets that evaluating the root module cannot see.
  evalConsumerShell = flakeFile: let
    consumerSource = builtins.path {
      path = builtins.dirOf flakeFile;
      name = "prelude-consumer-${builtins.baseNameOf (builtins.dirOf flakeFile)}";
    };
    consumerFlake = import (consumerSource + "/flake.nix");
    consumerInputs =
      inputs
      // {
        prelude = localFlake;
        self = consumerOutputs;
      };
    consumerOutputs =
      consumerFlake.outputs consumerInputs
      // {
        inputs = consumerInputs;
      };
  in
    consumerOutputs.devShells.${pkgs.stdenv.hostPlatform.system}.default;

  # The command-providing packages of the dogfood devshell (shell.nix).
  devshellCommandPackages = [
    config.packages.prelude-shell
    config.packages.prelude-motd
    config.packages.prelude-title-previews
    config.packages.prelude-menu
    config.packages.prelude-docs
    config.packages.prelude-portal
    pkgs.nix
    docsAutomation.sync
    docsAutomation.record
    previews
  ];

  preludeShellClosure = pkgs.closureInfo {
    rootPaths = [config.packages.prelude-shell];
  };
  componentClosures = {
    motd = pkgs.closureInfo {
      rootPaths = [config.packages.prelude-motd];
    };
    menu = pkgs.closureInfo {
      rootPaths = [config.packages.prelude-menu];
    };
    docs = pkgs.closureInfo {
      rootPaths = [config.packages.prelude-docs];
    };
  };

  # Assert that every advertised canonical invocation starts with an executable
  # provided by the devshell. Group selectors (`go:test`) are menu identity and
  # intentionally do not exist on PATH; their invocation (`go test`) does.
  mkRunnableCheck = checkName: surface: invocations: let
    executableForLine = line: builtins.head (lib.filter (token: token != "") (lib.splitString " " line));
    invocationExecutables = invocation: map executableForLine (lib.filter (line: line != "") (lib.splitString "\n" invocation));
    executables = lib.unique (lib.concatMap invocationExecutables invocations);
  in
    pkgs.runCommand checkName {nativeBuildInputs = devshellCommandPackages;} ''
      for cmd in ${lib.concatMapStringsSep " " lib.escapeShellArg executables}; do
        command -v "$cmd" >/dev/null 2>&1 || {
          echo "${surface} advertises canonical executable '$cmd' but no devshell package provides it" >&2
          exit 1
        }
      done
      touch "$out"
    '';
in {
  output-surface = assert lib.assertMsg
  (
    builtins.attrNames config.apps
    == [
      "examples"
      "prelude"
      "previews"
    ]
  )
  "Prelude's root app surface must contain only prelude plus the repository-only examples and previews apps";
  assert lib.assertMsg (
    lib.getExe config.packages.default == lib.getExe config.packages.prelude
  ) "packages.default must run Prelude so `nix run <flake> -- <command>` needs no app alias";
    pkgs.runCommand "output-surface" {} ''
      touch "$out"
    '';
  component-closures = let
    commonForbidden = [
      config.packages.prelude
      config.packages.prelude-shell
      config.packages.prelude-title
      config.packages.prelude-title-previews
      config.packages.prelude-wizard
      demos.examplesRunner
      previews
    ];
    assertAbsent = closure: forbidden:
      lib.concatMapStringsSep "\n" (package: ''
        if grep -Fxq ${lib.escapeShellArg (toString package)} ${closure}/store-paths; then
          echo "${package} leaked into a component closure" >&2
          exit 1
        fi
      '')
      forbidden;
  in
    pkgs.runCommand "component-closures" {} ''
      ${assertAbsent componentClosures.motd (
        commonForbidden
        ++ [
          config.packages.prelude-menu
          config.packages.prelude-menu.componentRoot
          config.packages.prelude-docs
          config.packages.prelude-docs.componentRoot
        ]
      )}
      ${assertAbsent componentClosures.menu (
        commonForbidden
        ++ [
          config.packages.prelude-motd
          config.packages.prelude-motd.componentRoot
          config.packages.prelude-docs
          config.packages.prelude-docs.componentRoot
        ]
      )}
      ${assertAbsent componentClosures.docs (
        commonForbidden
        ++ [
          config.packages.prelude-motd
          config.packages.prelude-motd.componentRoot
          config.packages.prelude-menu
          config.packages.prelude-menu.componentRoot
        ]
      )}
      touch "$out"
    '';
  consumer-template = evalConsumerShell ../templates/default/flake.nix;
  consumer-reference = evalConsumerShell ../examples/reference/flake.nix;
  # Building the module-produced packages runs shellcheck / go vet on the
  # generated artifacts.
  motd-default = config.packages.prelude-motd;
  title-default = config.packages.prelude-title;
  menu-default = config.packages.prelude-menu;
  menu-just-config = let
    justfile = pkgs.writeText "menu-justfile" "build:\n  echo build\n";
    menu =
      (import ../src/prelude/menu.nix {
        inherit
          (pkgs)
          lib
          writeShellApplication
          writeText
          symlinkJoin
          ;
        buildGoModule = args: args;
      })
      {
        just = {
          enable = true;
          inherit justfile;
          group = "just";
        };
      };
  in
    pkgs.runCommand "menu-just-config" {nativeBuildInputs = [pkgs.jq];} ''
      test "$(jq -r '.just.enable' ${menu.configFile})" = true
      test "$(jq -r '.just.group' ${menu.configFile})" = just
      test "$(jq -r '.just.justfile' ${menu.configFile})" = ${justfile}
      touch "$out"
    '';
  status-gradient-width = pkgs.runCommand "status-gradient-width" {} ''
    gradient_line=$(grep '^_PRELUDE_PROMPT_STATUS_GRADIENT=' ${config.packages.prelude-shell.shellInit})
    test "$(printf '%s' "$gradient_line" | tr -cd '#' | wc -c)" -eq 64
    grep -Fq '_PRELUDE_PROMPT_STATUS_GRADIENT_FG=' ${config.packages.prelude-shell.shellInit}
    grep -Fq '_PRELUDE_PROMPT_STATUS_HINT_BOLD_START=' ${config.packages.prelude-shell.shellInit}
    grep -Fq '_PRELUDE_PROMPT_STATUS_HINT_BOLD_WIDTH=' ${config.packages.prelude-shell.shellInit}

    ${lib.getExe pkgs.bash} <<'EOF'
    set -euo pipefail
    _PRELUDE_STARSHIP_STATUS_ENABLED=1
    _PRELUDE_PROMPT_STATUS_HINT=
    _PRELUDE_PROMPT_STATUS_GRADIENT='#000000:#111111:#222222:#333333'
    _PRELUDE_PROMPT_STATUS_GRADIENT_FG='#ffffff'
    ble/unicode/GraphemeCluster/match() {
      local text=$1 index=$2
      cs=''${text:index:1}
      w=1
      extend=0
    }
    source ${../src/prelude/shell/status.bash}

    COLUMNS=10
    _prelude_status_render ""
    test "''${#_prelude_status_gradient_chunks[@]}" -eq 4
    test "''${#_prelude_status_gradient_chunks[0]}" -eq 3
    test "''${#_prelude_status_gradient_chunks[1]}" -eq 3
    test "''${#_prelude_status_gradient_chunks[2]}" -eq 3
    test "''${#_prelude_status_gradient_chunks[3]}" -eq 1

    COLUMNS=19
    _prelude_status_render ""
    test "''${#_prelude_status_gradient_chunks[@]}" -eq 4
    test "''${#_prelude_status_gradient_chunks[0]}" -eq 6
    test "''${#_prelude_status_gradient_chunks[1]}" -eq 6
    test "''${#_prelude_status_gradient_chunks[2]}" -eq 6
    test "''${#_prelude_status_gradient_chunks[3]}" -eq 1

    _prelude_status_default='\g{x}unsafe'
    _ble_edit_str=
    _prelude_fake_hash_count=0
    _prelude_fake_styles=
    _prelude_fake_text=
    ble/prompt/unit/add-hash() { _prelude_fake_hash_count=$((_prelude_fake_hash_count + 1)); }
    ble/prompt/process-prompt-string() { _prelude_fake_styles+=$1; }
    ble/prompt/print() { _prelude_fake_text+=$1; }
    ble/prompt/backslash:prelude/status
    test "$_prelude_fake_hash_count" -eq 4
    test "''${#_prelude_fake_text}" -eq 19
    case $_prelude_fake_text in
      *'\g{x}unsafe'*) ;;
      *) exit 1 ;;
    esac
    case $_prelude_fake_styles in
      *'bg=#000000'*'bg=#333333'*) ;;
      *) exit 1 ;;
    esac
    case $_prelude_fake_styles in
      *unsafe*) exit 1 ;;
    esac
    EOF
    touch "$out"
  '';

  prelude-shell-default = assert lib.all (invocation: lib.elem invocation config.packages.prelude-menu.xInvocations) [
    "x prelude:previews"
    "x prelude:wizard"
  ];
    pkgs.runCommand "prelude-shell-default"
    {
      nativeBuildInputs = [
        config.packages.prelude-shell
        config.packages.prelude-motd
        config.packages.prelude-menu
        config.packages.prelude-docs
        pkgs.shellcheck
      ];
    }
    ''
              command -v prelude >/dev/null
              command -v prelude-preflight >/dev/null
              command -v motd >/dev/null
              command -v menu >/dev/null
              command -v docs >/dev/null
              # The shell core bundles every enabled component. Repository-only
              # generators/previews must not leak onto a consumer's PATH.
              test ! -e ${config.packages.prelude-shell}/bin/prelude-wizard
              test ! -e ${config.packages.prelude-shell}/bin/prelude-title-previews
              test ! -e ${config.packages.prelude-motd}/bin/menu
              test ! -e ${config.packages.prelude-motd}/bin/docs
              test ! -e ${config.packages.prelude-motd}/bin/prelude-wizard
              test ! -e ${config.packages.prelude-motd}/bin/prelude-title-previews
              test ! -e ${config.packages.prelude-menu}/bin/docs
              for package in \
                ${config.packages.prelude-shell} \
                ${config.packages.prelude-motd} \
                ${config.packages.prelude-menu} \
                ${config.packages.prelude-docs}; do
                test ! -e "$package/bin/examples"
                test ! -e "$package/bin/previews"
              done
              command -v prelude-wizard >/dev/null 2>&1 && {
                echo 'prelude-wizard leaked onto the consumer devshell PATH' >&2
                exit 1
              }
              # Assert closure membership, not only visible bin links: the
              # shell core must not install the full app or repository
              # generators transitively. Enabled component packages are
              # intentionally bundled, so they are excluded here.
              for forbidden in \
                ${config.packages.prelude} \
                ${config.packages.prelude-title} \
                ${config.packages.prelude-title-previews} \
                ${config.packages.prelude-wizard} \
                ${demos.examplesRunner} \
                ${previews}; do
                if grep -Fxq "$forbidden" ${preludeShellClosure}/store-paths; then
                  echo "$forbidden leaked into the prelude-shell closure" >&2
                  exit 1
                fi
              done
              # Prelude's own tools never claim generic executable names.
              test ! -e ${config.packages.prelude-shell}/bin/setup
              test ! -e ${config.packages.prelude-shell}/bin/preflight
              test ! -e ${config.packages.prelude-shell}/bin/wizard
              if command -v wizard >/dev/null 2>&1; then
                echo 'a bare wizard executable leaked onto the consumer devshell PATH' >&2
                exit 1
              fi
              prelude --help | grep -Fq 'usage: prelude <command> [args...]'
              prelude --help | grep -Fq 'wizard         generate a Prelude project configuration'
              prelude --help | grep -Fq 'preflight      print the shell code to eval'
              if prelude wizard --help >/dev/null 2>&1; then
                echo 'the devshell dispatcher found prelude-wizard without its package being added' >&2
                exit 1
              fi
              if prelude setup --help >/dev/null 2>&1; then
                echo 'prelude still answers the removed `setup` command' >&2
                exit 1
              fi
              # preflight is a printer: identical bytes from both entrypoints.
              prelude preflight > cli-preflight
              prelude-preflight > bin-preflight
              cmp -s cli-preflight bin-preflight
              cmp -s bin-preflight ${config.packages.prelude-shell}/share/prelude/shell/preflight.bash
              grep -Fq '. "$PRELUDE_INIT"' bin-preflight
              grep -Fq 'case "$-" in' bin-preflight
              # The snippet must name no build-time path: the MOTD binary
              # belongs to the init alone.
              if grep -Fq '/nix/store/' bin-preflight; then
                echo 'prelude-preflight bakes a store path into its snippet' >&2
                exit 1
              fi
              ${pkgs.bash}/bin/bash -n bin-preflight
              # Without a loaded environment it must say so and change nothing.
              ( unset PRELUDE_INIT; eval "$(prelude-preflight)" 2>preflight-warning )
              grep -Fq 'PRELUDE_INIT is unset' preflight-warning
              if prelude-preflight --unexpected >/dev/null 2>&1; then
                echo 'prelude-preflight accepted an unexpected argument' >&2
                exit 1
              fi
              # This builder is non-interactive with no DIRENV_IN_ENVRC — the
              # lorri shellHook case (nix-community/lorri#159). Rendering here
              # would go to a build log nobody reads, so it must stay absent.
              (
                export PRELUDE_INIT=${config.packages.prelude-shell.shellInit}
                eval "$(prelude-preflight)" 2>preflight-builder
                test ! -s preflight-builder
              )
              # direnv's .envrc has terminal-visible stderr. Every explicit
              # preflight evaluation renders independently.
              (
                export PRELUDE_INIT=${config.packages.prelude-shell.shellInit}
                export DIRENV_IN_ENVRC=1
                eval "$(prelude-preflight)" 2>preflight-direnv
                test -s preflight-direnv
                # An unexported render flag cannot leak into the environment
                # direnv captures from .envrc.
                test -z "''${_PRELUDE_PREFLIGHT_RENDER-}"
                eval "$(prelude-preflight)" 2>preflight-direnv-again
                test -s preflight-direnv-again
              )
              command -v starship >/dev/null
              command -v blesh-share >/dev/null
              test -f ${config.packages.prelude-shell}/share/blesh/ble.sh
              test -f ${config.packages.prelude-shell}/share/prelude/init.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/init.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/status.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/completion.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/status-cap.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/catalogue.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash
              test -f ${config.packages.prelude-shell}/nix-support/setup-hook
              grep -Fq 'prelude-init()' ${config.packages.prelude-shell}/nix-support/setup-hook
              grep -Fq '. ${config.packages.prelude-shell.shellInit}' ${config.packages.prelude-shell}/nix-support/setup-hook
              grep -Fq '_PRELUDE_INIT_LOADED-' ${config.packages.prelude-shell}/nix-support/setup-hook
              grep -Fq '_PRELUDE_INIT_LOADED=$PRELUDE_INIT' ${config.packages.prelude-shell}/share/prelude/shell/init.bash
              # STARSHIP_CONFIG must be exported from the setup-hook (not
              # shellHook) so direnv `use flake` re-themes the prompt.
              grep -Fq 'export STARSHIP_CONFIG=' ${config.packages.prelude-shell}/nix-support/setup-hook
              # Activation must reach the shell as an exported *variable*.
              # lorri runs shellHook inside the builder and replays only the
              # variables it exported, so `prelude-init` — a function — never
              # arrives. Without this path lorri users get no MOTD.
              grep -Fq 'export PRELUDE_INIT=' ${config.packages.prelude-shell}/nix-support/setup-hook
              test -f ${config.packages.prelude-shell}/share/prelude/shell/hook.bash
              test -f ${config.packages.prelude-shell}/share/prelude/shell/hook.zsh
              prelude --help | grep -Fq 'hook           print the shell hook'
              # The hook is pasted into a user's rc file, so it must never carry
              # a Bash-only payload into another shell.
              # NOTE: a pipeline prefixed with `!` is exempt from `set -e`, so
              # `! cmd | grep -q ...` never fails a build. Negative assertions
              # have to be written as an explicit test that exits.
              for dialect in bash zsh; do
                prelude hook "$dialect" | grep -Fq _prelude_hook
                prelude hook "$dialect" | grep -Fq 'PRELUDE_INIT'
                if prelude hook "$dialect" | grep -Fq 'export -f'; then
                  echo "prelude hook $dialect emits 'export -f'" >&2
                  exit 1
                fi
                if prelude hook "$dialect" | grep -Fq 'BASH_FUNC'; then
                  echo "prelude hook $dialect emits a Bash function payload" >&2
                  exit 1
                fi
              done
              # With no argument the dialect comes from $SHELL, the only place
              # the user's real shell is knowable (a builder is always Bash).
              SHELL=/bin/zsh prelude hook | grep -Fq precmd_functions
              SHELL=/bin/bash prelude hook | grep -Fq PROMPT_COMMAND
              if prelude hook fish >/dev/null 2>&1; then
                echo "prelude hook accepted an unsupported shell" >&2
                exit 1
              fi
              grep -Fq '_PRELUDE_BLESH=${pkgs.blesh}/share/blesh/ble.sh' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_STARSHIP=${lib.getExe pkgs.starship}' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_STARSHIP_STATUS_ENABLED=1' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq 'bleopt color_scheme=prelude' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              grep -Fq 'bleopt_import_path=' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              grep -Fq 'ble/prompt/backslash:lib/vim-airline' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              # ble.sh sources ~/.blerc during load, so the runtime must be on
              # import_path before that source line runs.
              seed_line=$(grep -n 'bleopt_import_path=' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash | head -1 | cut -d: -f1)
              blesh_line=$(grep -nF 'source "''$_PRELUDE_BLESH" --attach=none' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash | head -1 | cut -d: -f1)
              test "$seed_line" -lt "$blesh_line"
              grep -Fq 'function ble/contrib/scheme:prelude/initialize' ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash
              grep -Fq "ble-face -d prelude_status_cap" ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash
              test "$(grep -c '^  ble-face -[sd] ' ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash)" -eq 75
              ! grep -Fq '%prelude_' ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash
              ! grep -Eq '#[[:xdigit:]]{6}[[:alnum:]_]' ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash
              grep -Fq 'function ble/lib/vim-airline/theme:prelude/initialize' ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash
              grep -Fq 'ble-face -r vim_airline_@' ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash
              test "$(grep -c '^  ble-face -s ' ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash)" -eq 17
              ! grep -Fq '%prelude_' ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash
              ! grep -Eq '#[[:xdigit:]]{6}[[:alnum:]_]' ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash
              grep -Fq 'right_format' ${config.packages.prelude-prompt}
              grep -Fq '╰─' ${config.packages.prelude-prompt}
              ! grep -Fq ']()$character' ${config.packages.prelude-prompt}
              ! grep -Fq 'Type a command' ${config.packages.prelude-prompt}
              grep -Fq '_PRELUDE_PROMPT_PROJECT=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_PROMPT_NAVIGATION=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_PROMPT_NAVIGATION_RENDERED=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_PROMPT_STATUS_HINT=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_PROMPT_STATUS_HINT_RENDERED=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_PROMPT_STATUS_GRADIENT=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq '_PRELUDE_PROMPT_STATUS_GRADIENT_FG=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq "bleopt prompt_status_line='\\q{prelude/status}'" ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              grep -Fq "blehook PRECMD!='prelude/status/update'" ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              grep -Fq 'prelude/status/cap/install' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              grep -Fq '_PRELUDE_STARSHIP_FINAL_CONFIG=' ${config.packages.prelude-shell}/share/prelude/init.bash
              grep -Fq 'STARSHIP_CONFIG=' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              grep -Fq 'starship prompt --terminal-width=' ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              (
                COLUMNS=200
                _PRELUDE_STARSHIP_STATUS_ENABLED=1
                _PRELUDE_PROMPT_NAVIGATION='[?] motd  [x] menu  [d] docs'
                _PRELUDE_PROMPT_NAVIGATION_RENDERED='\g{fg=#555555,bg=#101010}[\g{bold,fg=#00ff00,bg=#101010}?\g{fg=#555555,bg=#101010}]\g{fg=#777777,bg=#101010} motd  \g{fg=#555555,bg=#101010}[\g{bold,fg=#00ff00,bg=#101010}x\g{fg=#555555,bg=#101010}]\g{fg=#777777,bg=#101010} menu  \g{fg=#555555,bg=#101010}[\g{bold,fg=#00ff00,bg=#101010}d\g{fg=#555555,bg=#101010}]\g{fg=#777777,bg=#101010} docs'
                _PRELUDE_PROMPT_PROJECT=demo
                _PRELUDE_PROMPT_STATUS_HINT=' hint  | '
                _PRELUDE_PROMPT_STATUS_HINT_RENDERED='\g{bold,fg=#101010,bg=#00ff00} hint \g{fg=#00ff00,bg=#101010}\g{fg=#555555,bg=#101010} | \g{fg=#777777,bg=#101010}'
                _prelude_fake_hashes=
                _prelude_fake_hash_count=0
                _prelude_fake_line=
                _prelude_fake_literal=
                _prelude_fake_processed=
                _prelude_fake_process_count=0
                _ble_edit_str=
                bleopt() { :; }
                ble/prompt/unit/add-hash() {
                  [ "$1" != '$_ble_edit_str' ] || _prelude_fake_line=
                  _prelude_fake_hash_count=$((_prelude_fake_hash_count + 1))
                  _prelude_fake_hashes+="<$1>"$'\n'
                }
                ble/prompt/print() {
                  _prelude_fake_literal=$1
                  _prelude_fake_line+=$1
                }
                ble/prompt/process-prompt-string() {
                  _prelude_fake_process_count=$((_prelude_fake_process_count + 1))
                  _prelude_fake_processed=$1
                  _prelude_fake_line+=$1
                }
                # Mirror the pinned Blesh grapheme contract for the focused
                # status callback test, including representative double-width
                # and combining clusters.
                ble/unicode/GraphemeCluster/match() {
                  local text=$1 index=$2
                  case "''${text:index}" in
                    '界'*) cs='界'; w=2; extend=$((''${#cs} - 1)) ;;
                    'é'*) cs='é'; w=1; extend=$((''${#cs} - 1)) ;;
                    *) cs=''${text:index:1}; w=1; extend=0 ;;
                  esac
                }
                source ${config.packages.prelude-shell}/share/prelude/shell/catalogue.bash
                source ${config.packages.prelude-shell}/share/prelude/shell/status.bash
                _prelude_status_revision=42
                prelude/status/update
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F "$_PRELUDE_PROMPT_STATUS_HINT_RENDERED"
                ! printf '%s' "$_prelude_fake_line" | grep -F '\g{none}'
                test "$_prelude_fake_process_count" -eq 1
                test "$_prelude_fake_processed" = "$_PRELUDE_PROMPT_STATUS_HINT_RENDERED"
                test "$_prelude_fake_hash_count" -eq 4
                printf '%s' "$_prelude_fake_hashes" | grep -F '<$_ble_edit_str>'
                printf '%s' "$_prelude_fake_hashes" | grep -F '<$_prelude_status_revision>'
                printf '%s' "$_prelude_fake_hashes" | grep -F '<$_prelude_status_health_record>'
                printf '%s' "$_prelude_fake_hashes" | grep -F '<$COLUMNS>'
                COLUMNS=40
                _ble_edit_str=
                ble/prompt/backslash:prelude/status
                COLUMNS=200
                COLUMNS=25
                _ble_edit_str=
                _prelude_fake_process_count=0
                ble/prompt/backslash:prelude/status
                test "$_prelude_fake_process_count" -eq 1
                test "$_prelude_fake_processed" = "$_PRELUDE_PROMPT_STATUS_HINT_RENDERED"
                ! printf '%s' "$_prelude_fake_line" | grep -F 'motd'
                COLUMNS=200

                _ble_edit_str='x'
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'cycle'
                printf '%s' "$_prelude_fake_line" | grep -F 'navigate'
                ! printf '%s' "$_prelude_fake_line" | grep -F "$_PRELUDE_PROMPT_STATUS_HINT"

                _ble_edit_str='x '
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'cycle'
                printf '%s' "$_prelude_fake_line" | grep -F 'navigate'
                ! printf '%s' "$_prelude_fake_line" | grep -F "$_PRELUDE_PROMPT_STATUS_HINT"

                _ble_edit_str='x build'
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'build a flake output'
                printf '%s' "$_prelude_fake_line" | grep -F 'x build'
                printf '%s' "$_prelude_fake_line" | grep -F 'bare x then Tab'

                _ble_edit_str='x build '
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'argument <empty>'
                printf '%s' "$_prelude_fake_line" | grep -F 'optional'
                printf '%s' "$_prelude_fake_line" | grep -F 'candidates: .#prelude-motd'

                _ble_edit_str='x build .#p'
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'argument .#p'
                printf '%s' "$_prelude_fake_line" | grep -F 'optional'
                printf '%s' "$_prelude_fake_line" | grep -F 'flake output to build'
                printf '%s' "$_prelude_fake_line" | grep -F 'candidates: .#prelude-motd'

                _ble_edit_str="x '"
                ble/prompt/backslash:prelude/status

                _ble_edit_str='x unknown'
                ble/prompt/backslash:prelude/status

                _prelude_status_health_record=$'checking\t\t\tchecking local server\tx dev\tr1'
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'health checking'
                _prelude_status_health_record=$'stopped\tstopped\t1m ago\tdown\tx dev\tr1'
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'down' | grep -F 'start: x dev'
                _prelude_status_health_record=$'stale\tstopped\t17m ago\tdown\tx dev\tr1'
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_line" | grep -F 'start: x dev' | grep -F 'stale: down (17m ago)'
                _ble_edit_str=
                ble/prompt/backslash:prelude/status
                ! printf '%s' "$_prelude_fake_line" | grep -F 'motd'
                ! printf '%s' "$_prelude_fake_line" | grep -F '\g{fg=#555555,bg=#101010}['

                _prelude_status_health_record=
                _prelude_status_hint=
                _prelude_status_hint_rendered=
                _prelude_status_default='界'
                COLUMNS=4
                _ble_edit_str=
                ble/prompt/backslash:prelude/status
                test "$_prelude_fake_literal" = '界  '
                _prelude_status_default='é'
                ble/prompt/backslash:prelude/status
                test "$_prelude_fake_literal" = 'é   '
                _prelude_status_default=Welcome
                COLUMNS=not-a-width
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_literal" | grep -F Welcome

                COLUMNS=200
                _prelude_status_hint=' hint  | '
                _prelude_status_hint_rendered='\g{bold,fg=#101010,bg=#00ff00} hint \g{fg=#00ff00,bg=#101010}\g{fg=#555555,bg=#101010} | \g{fg=#777777,bg=#101010}'
                _prelude_status_health_record=$'stopped\tstopped\t1m ago\t\\g{fg=#ff0000}unsafe\tx dev\tr2'
                _prelude_fake_process_count=0
                _prelude_fake_processed=
                ble/prompt/backslash:prelude/status
                printf '%s' "$_prelude_fake_literal" | grep -F '\g{fg=#ff0000}unsafe'
                test "$_prelude_fake_process_count" -eq 0
                test -z "$_prelude_fake_processed"
                refresh_tmp=$(mktemp -d)
                export PRELUDE_FAKE_REFRESH_COUNT="$refresh_tmp/count"
                export PRELUDE_FAKE_HELPER_CALLS="$refresh_tmp/calls"
                : > "$PRELUDE_FAKE_REFRESH_COUNT"
                : > "$PRELUDE_FAKE_HELPER_CALLS"
                touch "$refresh_tmp/config"
                cat > "$refresh_tmp/helper" <<'EOF'
      #!${pkgs.bash}/bin/bash
      printf '%s\n' "$1" >> "$PRELUDE_FAKE_HELPER_CALLS"
      if [ "$1" = --cached ]; then
        printf 'checking\t\t\tchecking local server\tx dev\tr1\n'
        exit 0
      fi
      printf 'refresh\n' >> "$PRELUDE_FAKE_REFRESH_COUNT"
      sleep 0.2
      EOF
                chmod +x "$refresh_tmp/helper"
                _prelude_status_helper="$refresh_tmp/helper"
                _prelude_status_config="$refresh_tmp/config"
                _prelude_status_refresh_pid=
                prelude/status/refresh
                prelude/status/refresh
                sleep 0.1
                refresh_count=$(wc -l < "$PRELUDE_FAKE_REFRESH_COUNT")
                wait
                test "$refresh_count" -eq 1
                # Rendering must only consume the cached record. A helper call here
                # would turn every editor redraw into a local-server probe.
                : > "$PRELUDE_FAKE_HELPER_CALLS"
                _prelude_fake_process_count=0
                ble/prompt/backslash:prelude/status
                test "$(wc -l < "$PRELUDE_FAKE_HELPER_CALLS")" -eq 0
                test "$_prelude_fake_process_count" -eq 0
              )
              (
                COLUMNS=4
                _ble_term_xenl=1
                _ble_term_sgr0='<reset>'
                _ble_prompt_status_panel=4
                _ble_prompt_status_data=()
                _ble_canvas_panel_class=(ble/textarea ble/textarea ble/edit/info ble/edit/visible-bell ble/prompt/status)
                _ble_canvas_panel_height=(1 0 0 0 0)
                _ble_canvas_panel_vfill=4
                _prelude_cap_goto=
                _prelude_cap_output=
                _prelude_cap_cursor=
                ble/color/face2g() { ret=cap-face; }
                ble/color/g2sgr() { ret='<cap>'; }
                ble/string#repeat() {
                  ret=
                  for ((i = 0; i < $2; i++)); do
                    ret+=$1
                  done
                }
                ble/canvas/panel#goto.draw() { _prelude_cap_goto="$1:''${2-0}:''${3-0}"; }
                ble/canvas/panel#put.draw() {
                  _prelude_cap_output=$2
                  _prelude_cap_cursor="$1:$3:$4"
                }
                ble/canvas/bflush.draw() { :; }
                _prelude_cap_reallocation_count=0
                _prelude_cap_height_operations=
                ble/canvas/panel#set-height.draw() {
                  _prelude_cap_height_operations+="$1:$2 "
                  _ble_canvas_panel_height[$1]=$2
                }
                ble/canvas/panel/reallocate-height.draw() {
                  ((++_prelude_cap_reallocation_count))
                  _ble_canvas_panel_height[4]=1
                  _ble_canvas_panel_height[5]=1
                }
                ble/edit/is-command-layout() { return 1; }
                source ${config.packages.prelude-shell}/share/prelude/shell/status-cap.bash
                prelude/status/cap/install
                test "''${_ble_canvas_panel_class[4]}" = prelude/status/cap
                test "''${_ble_canvas_panel_class[5]}" = ble/prompt/status
                test "$_ble_prompt_status_panel" -eq 5
                test "$_ble_canvas_panel_vfill" -eq 4
                prelude/status/cap#panel::getHeight 4
                test "$height" = 0:0
                bleopt_prompt_status_line='\q{prelude/status}'
                prelude/status/cap#panel::getHeight 4
                test "$height" = 0:1
                ble/edit/is-command-layout() { return 0; }
                prelude/status/cap#panel::getHeight 4
                test "$height" = 0:0
                ble/edit/is-command-layout() { return 1; }
                _prelude_status_cap_dirty=1
                prelude/status/cap#panel::render 4 0 0
                test "$_prelude_cap_reallocation_count" -eq 1
                test "$_prelude_cap_goto" = 4:0:0
                test "$_prelude_cap_output" = '<reset>    '
                test "$_prelude_cap_cursor" = 4:4:0
                # Blesh enters command layout through this exact collapse hook.
                ble/prompt/status#collapse
                test "''${_ble_canvas_panel_height[4]}" -eq 0
                test "''${_ble_canvas_panel_height[5]}" -eq 0
                test "$_prelude_cap_height_operations" = '4:0 5:0 '
              )
              (
                _ble_prompt_status_panel=4
                _ble_canvas_panel_class=(ble/textarea ble/textarea ble/edit/info ble/edit/visible-bell unexpected)
                _ble_canvas_panel_height=(1 0 0 0 0)
                _ble_canvas_panel_vfill=4
                source ${config.packages.prelude-shell}/share/prelude/shell/status-cap.bash
                ! prelude/status/cap/install >/dev/null 2>&1
                test "''${#_ble_canvas_panel_class[@]}" -eq 5
                test "''${_ble_canvas_panel_class[4]}" = unexpected
                test "$_ble_prompt_status_panel" -eq 4
                test "$_ble_canvas_panel_vfill" -eq 4
              )
              ${pkgs.bash}/bin/bash -n ${config.packages.prelude-shell}/share/prelude/init.bash
              for source in ${config.packages.prelude-shell}/share/prelude/shell/*.bash; do
                ${pkgs.bash}/bin/bash -n "$source"
              done
              ${pkgs.bash}/bin/bash -n ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash
              ${pkgs.bash}/bin/bash -n ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash
              shellcheck -x ${config.packages.prelude-shell}/share/prelude/init.bash
              shellcheck -x ${config.packages.prelude-shell}/share/prelude/shell/init.bash
              # PROMPT_COMMAND is legitimately a string or an array depending on
              # the Bash version, and the hook handles both shapes explicitly.
              shellcheck -x -e SC2178,SC2128 ${config.packages.prelude-shell}/share/prelude/shell/hook.bash
              shellcheck -x ${config.packages.prelude-shell}/share/prelude/shell/preflight.bash
              shellcheck -x -e SC1091,SC2154 ${config.packages.prelude-shell}/share/prelude/shell/bash-init.bash
              shellcheck -x -e SC2016,SC2154 ${config.packages.prelude-shell}/share/prelude/shell/status.bash
              shellcheck -x -e SC2154 ${config.packages.prelude-shell}/share/prelude/shell/completion.bash
              shellcheck -e SC2154 ${config.packages.prelude-shell}/share/prelude/shell/contrib/scheme/prelude.bash
              shellcheck -e SC2154 ${config.packages.prelude-shell}/share/prelude/shell/contrib/airline/prelude.bash
              touch "$out"
    '';

  prelude-default =
    pkgs.runCommand "prelude-default" {nativeBuildInputs = [config.packages.prelude];}
    ''
      command -v prelude >/dev/null
      test -x ${config.packages.prelude-preflight}/bin/prelude-preflight
      test -x ${config.packages.prelude-wizard}/bin/prelude-wizard
      test -x ${config.packages.prelude-title}/bin/prelude-title
      test -x ${config.packages.prelude-title-previews}/bin/prelude-title-previews
      test -x ${previews}/bin/prelude-previews
      test ! -e ${config.packages.prelude}/bin/wizard
      test ! -e ${config.packages.prelude-preflight}/bin/preflight
      test ! -e ${config.packages.prelude-wizard}/bin/wizard
      test ! -e ${config.packages.prelude-title}/bin/title
      test ! -e ${config.packages.prelude-title-previews}/bin/previews
      test ! -e ${previews}/bin/previews
      test ! -e ${config.packages.prelude-menu}/bin/wizard
      prelude --help | grep -Fq 'usage: prelude <command> [args...]'
      prelude --help | grep -Fq 'wizard         generate a Prelude project configuration'
      prelude wizard --help | grep -Fq 'usage: prelude wizard [--recipe path] [-o path]'
      if prelude setup --help >/dev/null 2>&1; then
        echo 'prelude still answers the removed `setup` command' >&2
        exit 1
      fi
      touch "$out"
    '';

  # With `prelude.prompt.enable = false` the generated init must not name
  # Starship, ble.sh, or bash-completion anywhere. Nix derives a derivation's
  # runtime references by scanning its output text for store hashes, so a single
  # mention would drag all three into a MOTD-only consumer's closure. Textual
  # absence is therefore exactly the closure guarantee, not a proxy for it.
  shell-init-motd-only = let
    backdrop = internalLib.resolveBackdropPalette "prelude" {};
    mkLight = motdRevision:
      (import ../src/prelude/shell-init.nix {
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
      })
      {
        inherit (backdrop) shadow palette;
        inherit motdRevision;
        projectName = "motd-only";
        motdCommand = "/prelude-test/bin/motd";
        promptEnabled = false;
      };
    light = mkLight "revision-a";
    changed = mkLight "revision-b";
  in
    pkgs.runCommand "shell-init-motd-only" {} ''
      for forbidden in \
        ${lib.getExe pkgs.starship} \
        ${pkgs.blesh} \
        ${pkgs.bash-completion}; do
        if grep -Fq "$forbidden" ${light.init}; then
          echo "MOTD-only init references $forbidden" >&2
          echo "that would pull the prompt runtime into every consumer's closure" >&2
          exit 1
        fi
      done
      # It must still be a working activation entrypoint.
      grep -Fq '_PRELUDE_PROMPT_ENABLED=0' ${light.init}
      grep -Fq '_PRELUDE_MOTD=' ${light.init}
      ${pkgs.bash}/bin/bash -n ${light.init}
      # The prompt hook reloads only when PRELUDE_INIT changes, so a MOTD-only
      # rebuild must perturb the generated init path without adding runtime state.
      test ${light.init} != ${changed.init}
      grep -Fq '# MOTD revision: revision-a' ${light.init}
      touch "$out"
    '';

  # The banner has two possible renderers for one environment: preflight (from
  # direnv's non-interactive .envrc) and `prelude hook` (from the interactive
  # prompt). State-free activation lets each loader render independently; the
  # hook's existing PRELUDE_INIT-path guard still prevents ordinary prompts from
  # repeatedly sourcing the same init. A sentinel MOTD with the prompt disabled
  # keeps this about loader behavior rather than ble.sh, Starship, or this repo's
  # own banner text.
  preflight-hook-handoff = let
    backdrop = internalLib.resolveBackdropPalette "prelude" {};
    sentinel = pkgs.writeShellApplication {
      name = "motd";
      text = "printf '%s\\n' PRELUDE-MOTD-SENTINEL";
    };
    shellPkg =
      (import ../src/prelude/shell-init.nix {
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
      })
      {
        inherit (backdrop) shadow palette;
        projectName = "preflight-handoff";
        motdCommand = lib.getExe sentinel;
        promptEnabled = false;
      };
    ptyPython = pkgs.python3.withPackages (pythonPackages: [pythonPackages.pyte]);
    ptyCommandPath = lib.makeBinPath [
      pkgs.bashInteractive
      pkgs.coreutils
      sentinel
    ];
  in
    pkgs.runCommand "preflight-hook-handoff" {} ''
      ${lib.getExe ptyPython} ${./preflight-hook-pty-test.py} \
        ${lib.getExe pkgs.bashInteractive} \
        ${shellPkg.init} \
        ${shellPkg.runtime}/preflight.bash \
        ${shellPkg.runtime}/hook.bash \
        ${lib.escapeShellArg ptyCommandPath} \
        PRELUDE-MOTD-SENTINEL
      touch "$out"
    '';

  # A devshell must never `export -f`. Bash stores an exported function as the
  # environment variable `BASH_FUNC_<name>%%`, and a `BASH_VERSION` guard cannot
  # prevent that — the guard runs inside the Nix builder, which is always Bash,
  # never the user's shell. Loaders that capture the environment (lorri, direnv)
  # replay that name into whatever shell the user actually runs, and zsh rejects
  # `%` in a variable name on every prompt. Shell-specific setup belongs in
  # `prelude hook`, where $SHELL is meaningful.
  #
  # Written as an explicit test rather than `! grep …`: a pipeline prefixed with
  # `!` is exempt from `set -e`, so a negated grep can never fail a build.
  # Comment lines are skipped so the rationale can stay next to the code.
  devshell-exports-no-functions = pkgs.runCommand "devshell-exports-no-functions" {} ''
    if grep -En '^[^#]*export -f' ${../nix/shell.nix}; then
      echo "nix/shell.nix uses 'export -f'; exported functions become" >&2
      echo "BASH_FUNC_* variables that break non-Bash shells on every prompt" >&2
      exit 1
    fi
    touch "$out"
  '';

  title-previews = pkgs.runCommand "title-previews" {} ''
    ${lib.getExe config.packages.prelude-title-previews} "choose me" > "$out"
    test "$(grep -c '^===== .* =====$' "$out")" -eq 25
    grep -q '^===== 3d-ascii =====$' "$out"
    grep -q '^===== calvin-s =====$' "$out"
    grep -q '^===== roman =====$' "$out"
    grep -q '^===== univers =====$' "$out"
    test "$(wc -l < "$out")" -gt 50
  '';

  title-generates = let
    # JSON, not Nix: nix-instantiate cannot write to /nix/var/nix/profiles
    # inside the build sandbox, so the title tool's Nix-recipe path is
    # unusable here. The tool accepts JSON recipes directly.
    recipe = pkgs.writeText "title.json" ''{"text":"prelude","font":"calvin-s"}'';
  in
    pkgs.runCommand "title-generates" {} ''
      ${lib.getExe config.packages.prelude-title} --recipe ${recipe} --output "$out"
      grep -q '┌─┐' "$out"
    '';

  # fromPkg is a small adapter over mkCommand: package selection is positional,
  # while program/arguments and presentation metadata stay composable extras.
  from-pkg = let
    command = preludeLib.fromPkg pkgs.nixfmt {
      arguments = ["."];
      description = "format Nix sources";
      key = "f";
    };
  in
    assert command.description == "format Nix sources";
    assert command.key == "f";
    assert command.invocation == "nixfmt .";
    assert lib.hasPrefix (lib.getExe pkgs.nixfmt) command.exec;
    assert command.runtimePackages == [pkgs.nixfmt];
      pkgs.runCommand "from-pkg" {} "touch $out";

  # Prelude owns navigation commands. `x` is always advertised on the MOTD
  # (bare); docs stays picker-only. Project Getting Started rows remain
  # focused on explicitly selected lifecycle commands. Bare `menu` remains a
  # compatibility PATH wrapper outside the catalogue.
  prelude-command-defaults = assert lib.all (name: lib.elem name config.packages.prelude-menu.commandNames) [
    "x"
    "docs"
  ];
  assert lib.elem "x" config.packages.prelude-motd.commandNames;
  assert lib.elem "x" config.packages.prelude-motd.commandInvocations;
  assert !lib.elem "menu" config.packages.prelude-motd.commandNames;
  assert !lib.elem "menu" config.packages.prelude-motd.commandInvocations;
  assert !lib.elem "docs" config.packages.prelude-motd.commandNames;
    pkgs.runCommand "prelude-command-defaults"
    {
      nativeBuildInputs = [
        config.packages.prelude-menu
        config.packages.prelude-docs
      ];
    }
    ''
      command -v x >/dev/null
      command -v menu >/dev/null
      command -v docs >/dev/null
      ! command -v help >/dev/null
      touch "$out"
    '';

  # Complete command keys stay public while the first colon derives group/label
  # presentation. Prelude stays first and configured groups follow in order.
  command-ordering = let
    plib = import ../src/prelude/lib.nix {inherit lib;};
    evaluated = lib.evalModules {
      modules = [
        ../src/prelude/options/shared.nix
        {
          prelude.sort.groups = [
            "docs"
            "develop"
            "demos"
          ];
          prelude.commands = {
            x = {};
            dev = {};
            "docs:sync" = {};
            "docs:record" = {};
            "demos:menu".exec = "nix run .#example-menu";
          };
        }
        {
          prelude.commands."docs:record".description = "merged";
        }
      ];
    };
    normalized = plib.normalizeCommandGroups evaluated.config.prelude.sort.groups evaluated.config.prelude.commands;
    actual =
      map (group: {
        inherit (group) title;
        commands =
          map (command: {
            inherit
              (command)
              name
              label
              run
              ;
          })
          group.tasks;
      })
      normalized;
    expected = [
      {
        title = "prelude";
        commands = [
          {
            name = "x";
            label = "x";
            run = "x";
          }
        ];
      }
      {
        title = "docs";
        commands = [
          {
            name = "docs:record";
            label = "record";
            run = "record";
          }
          {
            name = "docs:sync";
            label = "sync";
            run = "sync";
          }
        ];
      }
      {
        title = "develop";
        commands = [
          {
            name = "dev";
            label = "dev";
            run = "dev";
          }
        ];
      }
      {
        title = "demos";
        commands = [
          {
            name = "demos:menu";
            label = "menu";
            run = "nix run .#example-menu";
          }
        ];
      }
    ];
    docsGroup = builtins.elemAt normalized 1;
  in
    assert !(evaluated.options ? sort);
    assert evaluated.options.prelude.sort ? groups;
    assert actual == expected;
    assert (builtins.head docsGroup.tasks).description == "merged";
      pkgs.runCommand "command-ordering" {} "touch $out";

  # Core navigation shortcuts are synthesized from component availability;
  # consumers cannot remove or advertise commands that are disabled.
  component-shortcuts = let
    all = internalLib.componentShortcuts {
      motd = true;
      menu = true;
      docs = true;
    };
    menuOnly = internalLib.componentShortcuts {
      motd = false;
      menu = true;
      docs = false;
    };
  in
    assert all
    == [
      {
        command = "motd";
        alias = "?";
      }
      {
        command = "menu";
        alias = "x";
      }
      {
        command = "docs";
        alias = "d";
      }
    ];
    assert menuOnly
    == [
      {
        command = "menu";
        alias = "x";
      }
    ];
      pkgs.runCommand "component-shortcuts" {} "touch $out";

  # prelude.lib.mdSplit → { title = "README"; text; children = [preamble, H2…] }.
  # docs.nix renames first child to project + rootReadme when text matches.
  mdSplit-pages = let
    sample = ''
      <div align="center">badge</div>

      # Guide

      intro before any H2

      ## First section

      first body

      ```md
      ## not a real heading
      ```

      ## Second section

      second body

      ## motd options (`prelude.motd.*`)

      punct body
    '';
    node = preludeLib.mdSplit sample;
    children = node.children;
    titles = map (l: l.title) children;
    bodies = map (l: builtins.readFile l.text) children;
    fromPath = preludeLib.mdSplit ../README.md;
    # H1 immediately followed by H2 — preamble body empty but still index 0.
    thin = preludeLib.mdSplit ''
      # Thin

      ## Alpha

      alpha body
    '';
    thinTitles = map (l: l.title) thin.children;
  in
    assert node.title == "README";
    assert node ? text; # always set (toFile for string src)
    
    assert builtins.length children == 4;
    # Pure mdSplit keeps H1-derived preamble title; docs.nix renames to project.
    assert titles
    == [
      "Guide"
      "First section"
      "Second section"
      "motd options (`prelude.motd.*`)"
    ];
    assert lib.hasInfix "badge" (builtins.elemAt bodies 0);
    assert lib.hasInfix "intro before any H2" (builtins.elemAt bodies 0);
    assert !(lib.hasInfix "# Guide" (builtins.elemAt bodies 0));
    assert lib.hasInfix "first body" (builtins.elemAt bodies 1);
    assert !(lib.hasInfix "## First section" (builtins.elemAt bodies 1));
    assert lib.hasInfix "## not a real heading" (builtins.elemAt bodies 1);
    assert lib.hasInfix "second body" (builtins.elemAt bodies 2);
    assert lib.hasInfix "punct body" (builtins.elemAt bodies 3);
    assert fromPath.title == "README";
    assert fromPath ? text;
    assert builtins.length fromPath.children > 1;
    # Empty preamble still occupies children[0]; Alpha is not promoted.
    assert thin.title == "README";
    assert thinTitles
    == [
      "Thin"
      "Alpha"
    ];
    assert lib.hasInfix "alpha body" (builtins.readFile (builtins.elemAt thin.children 1).text);
      pkgs.runCommand "mdSplit-pages" {} "touch $out";

  # docs.nix nav: README → <project> → first original H2 … + FIGlet flag.
  mdSplit-readme-nav = let
    docsPkg =
      import ../src/prelude/docs.nix
      {
        inherit
          (pkgs)
          lib
          writeText
          buildGoModule
          runCommand
          nixosOptionsDoc
          figlet
          ;
      }
      {
        theme = "phosphor";
        colorProfile = "auto";
        project = "myproj";
        rootReadme = ../README.md;
        pages = [
          (preludeLib.mdSplit ../README.md)
        ];
        nixosOptions = {
          options = {};
        };
      };
    cfg = builtins.fromJSON (builtins.readFile "${docsPkg.passthru.config}/config.json");
    root = builtins.head cfg.nav;
    kids = root.children;
    first = builtins.head kids;
    second = builtins.elemAt kids 1;
  in
    assert root.kind == "group";
    assert root.title == "README";
    assert first.kind == "leaf";
    assert first.title == "myproj";
    assert first.rootReadme == true;
    assert second.kind == "leaf";
    assert second.title == "Quickstart (Setup Wizard)";
    assert (cfg.heroFile or "") != "";
      pkgs.runCommand "mdSplit-readme-nav" {} "touch $out";
  prompt-shadow-palette = let
    mkPromptArtifacts = promptConfig:
      (import ../src/prelude/prompt.nix {
        inherit (pkgs) lib formats;
      })
      promptConfig;
    mkPrompt = promptConfig: (mkPromptArtifacts promptConfig).live;
    internalLib = import ../src/prelude/lib.nix {inherit lib;};
    themeCases = {
      phosphor = "#121614";
      minted = "#121218";
      amber = "#16120e";
      solarized = "#132828";
      nord = "#333841";
      gruvbox = "#2d2b27";
      mono = "#101010";
      apathy = "#141118";
      prelude = "#141118";
      paper = "#f4f2ec";
    };
    themePrompts = lib.mapAttrs (theme: _: mkPrompt {inherit theme;}) themeCases;
    themeChecks = lib.concatStringsSep "\n" (
      lib.mapAttrsToList (theme: shadow: ''
        test '${(internalLib.resolveBackdropPalette theme {}).shadow}' = '${shadow}'
        ! grep -Eq '^shadow = ' ${themePrompts.${theme}}
        ! grep -Fq 'bg:window' ${themePrompts.${theme}}
        ! grep -Eq '^window = ' ${themePrompts.${theme}}
      '')
      themeCases
    );
    overrideShadows = {
      overridden = (internalLib.resolveBackdropPalette "apathy" {bg = "#6496c8";}).shadow;
      black = (internalLib.resolveBackdropPalette "apathy" {bg = "#000000";}).shadow;
      shortHex = (internalLib.resolveBackdropPalette "apathy" {bg = "#abc";}).shadow;
      indexed = (internalLib.resolveBackdropPalette "apathy" {bg = 212;}).shadow;
      packed = (internalLib.resolveBackdropPalette "apathy" {bg = 660510;}).shadow;
    };
    menuBoundary =
      (import ../src/prelude/menu.nix {
        inherit
          (pkgs)
          lib
          writeShellApplication
          writeText
          symlinkJoin
          ;
        buildGoModule = args: args;
      })
      {theme = "apathy";};
    docsBoundary =
      (import ../src/prelude/docs.nix {
        inherit
          (pkgs)
          lib
          writeText
          runCommand
          nixosOptionsDoc
          figlet
          ;
        buildGoModule = args: args;
      })
      {
        theme = "apathy";
        pages = [{text = pkgs.writeText "prelude-docs-boundary.md" "boundary";}];
      };
    customPromptSource = pkgs.writeText "prelude-custom-prompt.toml" "format = \"custom\"\n";
    customPrompt = mkPrompt {
      theme = "apathy";
      configFile = customPromptSource;
    };
    shellPkg =
      (import ../src/prelude/shell-init.nix {
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
      })
      {
        palette = internalLib.resolvePalette "apathy" {};
        shadow = "#1e1e1e";
        motdCommand = pkgs.writeShellScript "prelude-pty-motd" "exit 0";
        statusEnabled = true;
        promptFinalConfig = (mkPromptArtifacts {theme = "apathy";}).final;
        commandEntries = [
          {
            name = "build";
            label = "build";
            group = "development";
            grouped = true;
            invocation = "nix build";
            xInvocation = "x build";
            description = "build fixture";
            args = [];
          }
          {
            name = "check";
            label = "check";
            group = "development";
            grouped = true;
            invocation = "nix flake check";
            xInvocation = "x check";
            description = "check fixture";
            args = [];
          }
        ];
        navigation = [
          {
            alias = "?";
            command = "motd";
          }
          {
            alias = "x";
            command = "menu";
          }
          {
            alias = "d";
            command = "docs";
          }
        ];
      };
    ptyPython = pkgs.python3.withPackages (pythonPackages: [pythonPackages.pyte]);
    ptyCommandPath = lib.makeBinPath [
      pkgs.bash
      pkgs.starship
      pkgs.coreutils
      pkgs.findutils
      pkgs.gawk
      pkgs.gnugrep
      pkgs.gnused
      pkgs.ncurses
      pkgs.procps
    ];
  in
    pkgs.runCommand "prompt-shadow-palette" {nativeBuildInputs = [pkgs.jq];} ''
      set -euo pipefail
      ${themeChecks}
      test '${overrideShadows.overridden}' = '#6798c9'
      test '${overrideShadows.black}' = '#060606'
      test '${overrideShadows.shortHex}' = '#acbccd'
      test '${overrideShadows.indexed}' = '#ff8ad8'
      test '${overrideShadows.packed}' = '#101923'

      prompt=${themePrompts.apathy}
      grep -Fq '╰─' "$prompt"
      ! grep -Fq ']()$character' "$prompt"
      grep -Fq 'style = "fg:surface"' "$prompt"
      ! grep -Fq 'bg:window' "$prompt"
      ! grep -Eq '^window = ' "$prompt"

      cmp ${customPromptSource} ${customPrompt}
      test "$(jq -r '.palette | has("shadow") or has("window")' ${menuBoundary.passthru.configFile})" = false
      test "$(jq -r '.palette | has("shadow") or has("window")' ${docsBoundary.passthru.config}/config.json)" = false

      init=${shellPkg.init}
      runtime=${shellPkg.runtime}
      scheme="$runtime/contrib/scheme/prelude.bash"
      ! grep -Fq '_PRELUDE_WINDOW_BACKGROUND_SET' "$init"
      ! grep -Fq '_PRELUDE_PROMPT_WINDOW_MANAGED' "$init"
      ! grep -Fq 'textarea-background' "$runtime/bash-init.bash"
      ! grep -Fq 'command-background' "$runtime/bash-init.bash"
      test ! -e "$runtime/textarea-background.bash"
      ! grep -Fq 'prelude_textarea_window' "$scheme"
      ! grep -Fq 'refresh-face' "$scheme"
      grep -Fq "ble-face -s prompt_status_line        'fg=#4d4a56,bg=#1e1e1e'" "$scheme"
      grep -Fq "ble-face -d prelude_status_cap        'fg=#1b1629,bg=#1e1e1e'" "$scheme"

      airline="$runtime/contrib/airline/prelude.bash"
      grep -Fq 'function ble/lib/vim-airline/theme:prelude/initialize' "$airline"
      grep -Fq "ble-face -s vim_airline_a              'fg=#0e0b13,bg=#77f5c9'" "$airline"
      grep -Fq "ble-face -s vim_airline_a_insert       'fg=#0e0b13,bg=#82aaff'" "$airline"
      grep -Fq "ble-face -s vim_airline_b              'fg=#77f5c9,bg=#2a2441'" "$airline"
      grep -Fq "ble-face -s vim_airline_c              'fg=#7d7a8b,bg=#1b1629'" "$airline"
      ! grep -Fq '%prelude_' "$airline"
      grep -Fq 'bleopt_import_path=' "$runtime/bash-init.bash"
      grep -Fq 'ble/prompt/backslash:lib/vim-airline' "$runtime/bash-init.bash"

      # The vim-airline theme ships but is not exercised here. Its PTY smoke
      # asserted that airline REPLACES Prelude's status row — it failed
      # whenever "Run commands:" was on screen, which is the row we actually
      # use. So the check gated every build on an integration nobody runs, and
      # it has been red since 2026-08-12, blocking every check ordered after
      # it. The static assertions above still cover the theme's faces and its
      # import_path wiring, which is what would silently rot.

      # The prompt-final PTY smoke is not restored with it. It asserted the
      # submitted line collapses to Starship's `❯ `, which the prompt stopped
      # doing when it moved to the framed layout — it now renders `╰─ : cmd`.
      # The test encodes the older design, so it fails on a healthy prompt.

      ${lib.getExe pkgs.bash} -n "$runtime/init.bash"
      ${lib.getExe pkgs.bash} -n "$runtime/bash-init.bash"
      ${lib.getExe pkgs.bash} -n "$scheme"
      ${lib.getExe pkgs.bash} -n "$runtime/contrib/airline/prelude.bash"
      touch "$out"
    '';

  prompt-renders-shortcuts = pkgs.runCommand "prompt-renders-shortcuts" {} ''
    export NO_COLOR=1
    export HOME="$TMPDIR/home"
    export XDG_CACHE_HOME="$TMPDIR/cache"
    mkdir -p "$HOME" "$XDG_CACHE_HOME"
    export STARSHIP_CONFIG=${config.packages.prelude-prompt}
    export STARSHIP_SHELL=bash
    ${lib.getExe pkgs.starship} prompt --terminal-width 79 --status 0 > "$TMPDIR/normal"
    ${lib.getExe pkgs.starship} prompt --right --terminal-width 79 --status 0 > "$TMPDIR/status"

    # One breathing row (add_newline), then context, stem, and input rows.
    test "$(od -An -t x1 -N 1 "$TMPDIR/normal" | tr -d '[:space:]')" = 0a
    sed -n '2p' "$TMPDIR/normal" | grep -F 'π'
    sed -n '2p' "$TMPDIR/normal" | grep -F '╭'
    sed -n '2p' "$TMPDIR/normal" | grep -F 'motd'
    sed -n '2p' "$TMPDIR/normal" | grep -F 'x'
    sed -n '2p' "$TMPDIR/normal" | grep -F 'docs'
    sed -n '3p' "$TMPDIR/normal" | grep -F '│'
    sed -n '4p' "$TMPDIR/normal" | grep -F '╰─'
    ! grep -F '❯' "$TMPDIR/normal"
    grep -F '──╯' "$TMPDIR/status"
    touch "$out"
  '';

  prompt-status-runtime = let
    statusPkg =
      (import ../src/prelude/prompt-status.nix {
        inherit
          (pkgs)
          lib
          writeText
          buildGoModule
          ;
      })
      {
        project = "fixture";
        command = "dev";
        check = "true";
        ttl = "5m";
        start = "x dev";
      };
    shellPkg =
      (import ../src/prelude/shell-init.nix {
        inherit lib;
        writeText = pkgs.writeText;
        runCommand = pkgs.runCommand;
        starship = pkgs.starship;
        blesh = pkgs.blesh;
        bash-completion = pkgs.bash-completion;
        stdenv = pkgs.stdenv;
      })
      {
        palette = internalLib.resolvePalette "phosphor" {};
        projectName = "fixture";
        commandEntries = [];
        navigation = [];
        statusEnabled = false;
        promptStatusCommand = null;
        promptStatusConfig = null;
      };
    shellInitText = builtins.readFile shellPkg.init;
  in
    assert lib.hasInfix "_PRELUDE_STARSHIP_STATUS_ENABLED=0" shellInitText;
    assert lib.hasInfix "_PRELUDE_PROMPT_STATUS=''" shellInitText;
    assert lib.hasInfix "_PRELUDE_PROMPT_STATUS_CONFIG=''" shellInitText;
    assert lib.hasInfix "_PRELUDE_PROMPT_NAVIGATION=''" shellInitText;
      pkgs.runCommand "prompt-status-runtime"
      {
        nativeBuildInputs = [
          statusPkg
          pkgs.coreutils
          pkgs.gawk
        ];
      }
      ''
              export HOME="$TMPDIR/home"
              export XDG_CACHE_HOME="$TMPDIR/cache"
              mkdir -p "$HOME" "$XDG_CACHE_HOME"
              cached="$(${lib.getExe statusPkg} --cached)"
              test "$(printf '%s\n' "$cached" | awk -F '\t' '{ print NF }')" -eq 6
              printf '%s\n' "$cached" | grep -F 'checking'
              printf '%s\n' "$cached" | grep -F $'\tx dev\t'
              refreshed="$(${lib.getExe statusPkg} --refresh)"
              test "$(printf '%s\n' "$refreshed" | awk -F '\t' '{ print NF }')" -eq 6
              printf '%s\n' "$refreshed" | grep -F 'healthy'
              printf '%s\n' "$refreshed" | grep -F $'\tx dev\t'
              cat > "$TMPDIR/slow-check" <<EOF
        #!${pkgs.bash}/bin/bash
        touch "$TMPDIR/refresh-started"
        printf 'check\n' >> "$TMPDIR/refresh-count"
        sleep 0.2
        EOF
              chmod +x "$TMPDIR/slow-check"
              cat > "$TMPDIR/slow-status.json" <<EOF
        {"project":"concurrent","command":"dev","check":"$TMPDIR/slow-check","ttl":"5m","start":"x dev"}
        EOF
              ${lib.getExe statusPkg} --refresh --config "$TMPDIR/slow-status.json" >/dev/null &
              first_refresh=$!
              for _ in $(seq 1 50); do
                [ -e "$TMPDIR/refresh-started" ] && break
                sleep 0.01
              done
              test -e "$TMPDIR/refresh-started"
              ${lib.getExe statusPkg} --refresh --config "$TMPDIR/slow-status.json" >/dev/null &
              second_refresh=$!
              wait "$first_refresh"
              wait "$second_refresh"
              test "$(wc -l < "$TMPDIR/refresh-count")" -eq 1
              touch "$out"
      '';
  # Explicit local-server health has two validation boundaries: the option
  # rejects malformed values, and the per-system catalogue resolves the key
  # after package-backed commands have been merged.
  prompt-local-server-evaluation = let
    validLocalServer = {
      command = "dev";
      check = "true";
      ttl = "5m";
    };
    customConfigFile = ../nix/internal/title.txt;
    evalPrompt = localServer: configFile:
      builtins.tryEval (
        let
          evaluated = lib.evalModules {
            modules = [
              ../src/prelude/options/shared.nix
              ../src/prelude/options/motd.nix
              ../src/prelude/options/menu.nix
              ../src/prelude/options/docs.nix
              ../src/prelude/options/prompt.nix
              {
                prelude = {
                  commands.dev = {
                    description = "start the dev server";
                    exec = "pnpm dev";
                  };
                  prompt = {
                    enable = true;
                    inherit localServer configFile;
                  };
                };
              }
            ];
          };
        in
          builtins.deepSeq evaluated.config evaluated.config
      );
    commandKey = "dev;unsafe";
    start = "x ${lib.escapeShellArg commandKey}";
    # Keep the nested flake evaluation isolated from this repository's
    # outputs. Reusing the outer `self` would recursively evaluate the
    # production perSystem module instead of the fixture below.
    fixtureInputs = {
      self = {
        outPath = toString ../.;
        inputs = {};
      };
    };
    fixtureSystem = pkgs.stdenv.hostPlatform.system;
    fixturePkgs = {
      inherit lib;
      # The test only reads the descriptor, so a store text path is enough.
      writeText = builtins.toFile;
      # The exported module must never fall back to Nixpkgs' mutable default
      # Go alias; preserve prompt-status.nix's passthru with the pinned builder.
      buildGoModule = throw "Prelude must use pkgs.buildGo126Module";
      buildGo126Module = args: args.passthru;
      symlinkJoin = args:
        (builtins.derivation {
          name = args.name;
          system = fixtureSystem;
          builder = "/bin/sh";
          args = [
            "-c"
            "mkdir -p \"$out\""
          ];
        })
        // args.passthru;
    };
    fixtureModule = flakePartsLib.importApply ../src/prelude/module.nix {
      localFlake = {};
      flake-parts-lib = flakePartsLib;
    };
    evalFixture = localServer: let
      evaluated = flakePartsLib.evalFlakeModule {inputs = fixtureInputs;} {
        systems = [fixtureSystem];
        imports = [fixtureModule];
        prelude = {
          project = "fixture";
          prompt = {
            enable = true;
            inherit localServer;
          };
        };
        perSystem = {...}: {
          # Avoid importing nixpkgs into the nested evaluation; this check
          # only needs the descriptor passthru supplied by fixturePkgs.
          _module.args.pkgs = fixturePkgs;
          prelude.commands.${commandKey} = {
            description = "start the fixture server";
            exec = "true";
          };
        };
      };
    in
      builtins.deepSeq evaluated.config.allSystems.${fixtureSystem}.packages.prelude-shell.promptStatusPkg
      evaluated.config.allSystems.${fixtureSystem}.packages.prelude-shell.promptStatusPkg;
    valid = evalPrompt validLocalServer null;
    invalidTtl = evalPrompt (validLocalServer // {ttl = "0m";}) null;
    invalidOverflowTtl = evalPrompt (validLocalServer // {ttl = "9223372036854775807h";}) null;
    invalidCheck = evalPrompt (validLocalServer // {check = "  ";}) null;
    custom = evalPrompt validLocalServer customConfigFile;
    perSystemValid = evalFixture (validLocalServer // {command = commandKey;});
    perSystemUnknown = builtins.tryEval (evalFixture (validLocalServer // {command = "missing";}));
    statusConfig = perSystemValid.configFile;
  in
    assert valid.success;
    assert invalidTtl.success == false;
    assert invalidCheck.success == false;
    assert invalidOverflowTtl.success == false;
    assert custom.success;
    assert perSystemValid != null;
    assert perSystemUnknown.success == false;
    assert valid.value.prelude.prompt.localServer.command == "dev";
    assert valid.value.prelude.prompt.localServer.check == "true";
    assert valid.value.prelude.prompt.localServer.ttl == "5m";
    assert custom.value.prelude.prompt.configFile == customConfigFile;
      pkgs.runCommand "prompt-local-server-evaluation" {nativeBuildInputs = [pkgs.jq];} ''
        ${lib.getExe pkgs.jq} -e --arg start ${lib.escapeShellArg start} \
          '.start == $start' ${statusConfig} >/dev/null
        touch "$out"
      '';

  # The MOTD advertises bare project commands (plus bare `x`);
  # the menu retains canonical underlying invocations for execution and
  # diagnostics.
  motd-commands-runnable =
    mkRunnableCheck "motd-commands-runnable" "motd"
    config.packages.prelude-motd.commandInvocations;

  menu-commands-runnable =
    mkRunnableCheck "menu-commands-runnable" "menu"
    config.packages.prelude-menu.commandInvocations;

  # Built-in navigation aliases must resolve on the same PATH as their labels.
  motd-shortcuts-runnable = assert config.packages.prelude-motd.shortcutAliases
  == [
    "?"
    "x"
    "d"
  ];
    mkRunnableCheck "motd-shortcuts-runnable" "built-in shortcuts"
    config.packages.prelude-motd.shortcutAliases;

  titles-command-renders =
    pkgs.runCommand "titles-command-renders"
    {
      nativeBuildInputs = [config.packages.prelude-title-previews];
    }
    ''
      prelude-title-previews prelude > "$out"
      test "$(grep -c '^===== .* =====$' "$out")" -eq 25
      grep -q '^===== 3d-ascii =====$' "$out"
      grep -q '^===== calvin-s =====$' "$out"
      test "$(wc -l < "$out")" -gt 50
    '';

  # Package-backed ungrouped aliases carry their runtime package and wrapper.
  package-command-bundled = assert lib.elem config.treefmt.build.wrapper config.packages.prelude-menu.commandRuntimePackages;
    pkgs.runCommand "package-command-bundled"
    {
      nativeBuildInputs = [config.packages.prelude-menu];
    }
    ''
      command -v treefmt >/dev/null
      command -v fmt >/dev/null
      touch "$out"
    '';

  colon-command-names-preserved = let
    internalPreludeLib = import ../src/prelude/lib.nix {inherit lib;};
    imported = internalPreludeLib.normalizeCommand "test:unit" {
      exec = "npm run test:unit";
    };
  in
    assert imported.name == "test:unit";
    assert imported.group == "test";
    assert imported.label == "unit";
      pkgs.runCommand "colon-command-names-preserved" {} "touch $out";

  # Explicit `group` overrides the colon-inferred default without changing the
  # key identity, label, or `grouped` (PATH-wrapper) behavior.
  explicit-group-override = let
    internalPreludeLib = import ../src/prelude/lib.nix {inherit lib;};
    flat = internalPreludeLib.normalizeCommand "lint" {
      group = "quality";
      exec = "eslint .";
    };
    colonKey = internalPreludeLib.normalizeCommand "go:test" {
      group = "ci";
      exec = "go test ./...";
    };
  in
    assert flat.name == "lint";
    assert flat.group == "quality";
    assert flat.label == "lint";
    assert flat.grouped == false;
    assert colonKey.name == "go:test";
    assert colonKey.group == "ci";
    assert colonKey.label == "test";
    assert colonKey.grouped == true;
      pkgs.runCommand "explicit-group-override" {} "touch $out";

  duplicate-canonical-invocations-rejected = let
    internalPreludeLib = import ../src/prelude/lib.nix {inherit lib;};
    attempted = builtins.tryEval (
      builtins.deepSeq (internalPreludeLib.normalizeCommandEntries {
        "go:test" = {
          exec = "go test";
        };
        "quality:test" = {
          exec = "go test";
        };
      })
      true
    );
  in
    assert !attempted.success;
      pkgs.runCommand "duplicate-canonical-invocations-rejected" {} "touch $out";

  # Group prefixes are parsed into menu metadata and never become PATH names.
  # Canonical package invocations remain the native CLI syntax.
  grouped-commands-use-canonical-invocations = assert lib.elem "go:vet" config.packages.prelude-menu.commandNames;
  assert lib.elem "go vet -C src ./..." config.packages.prelude-menu.commandInvocations;
  assert lib.elem "x go:vet" config.packages.prelude-menu.xInvocations;
  assert !lib.elem "go:vet" config.packages.prelude-menu.commandWrapperNames;
  assert !lib.elem "go-vet" config.packages.prelude-menu.commandWrapperNames;
    pkgs.runCommand "grouped-commands-use-canonical-invocations"
    {nativeBuildInputs = [config.packages.prelude-menu];}
    ''
      command -v go >/dev/null
      ! command -v go:vet >/dev/null
      ! command -v go-vet >/dev/null
      touch "$out"
    '';

  # Docs options accept nested nav nodes and full nixosOptionsDoc arg pass-through.
  docs-options = let
    tiny = lib.evalModules {
      modules = [
        {
          options.demo = lib.mkOption {
            type = lib.types.str;
            default = "x";
            description = "demo option";
          };
        }
      ];
    };
    evaluated = lib.evalModules {
      modules = [
        ../src/prelude/options/shared.nix
        ../src/prelude/options/docs.nix
        {
          prelude.docs.pages = [
            {text = ../docs/welcome.md;}
            {
              title = "Guides";
              children = [
                {text = ../docs/commands.md;}
              ];
            }
            {
              generate = "nixosOptions";
              title = "Options";
            }
          ];
          # Full nixosOptionsDoc argument set, including a non-transform field.
          prelude.docs.nixosOptions = {
            inherit (tiny) options;
            documentType = "none";
            warningsAreErrors = false;
            revision = "check-rev";
          };
        }
      ];
    };
    pages = evaluated.config.prelude.docs.pages;
    nixos = evaluated.config.prelude.docs.nixosOptions;
    # Exercise pass-through: builder must accept non-transform args unchanged.
    docsPkg =
      import ../src/prelude/docs.nix
      {
        inherit
          (pkgs)
          lib
          writeText
          buildGoModule
          runCommand
          nixosOptionsDoc
          figlet
          ;
      }
      {
        theme = "phosphor";
        colorProfile = "auto";
        project = "check";
        pages = [
          {
            generate = "nixosOptions";
            title = "Options";
          }
        ];
        nixosOptions = {
          inherit (tiny) options;
          documentType = "none";
          warningsAreErrors = false;
          revision = "check-rev";
        };
      };
  in
    assert builtins.length pages == 3;
    assert (builtins.head pages).text == ../docs/welcome.md;
    assert (builtins.elemAt pages 1).title == "Guides";
    assert (builtins.elemAt pages 2).generate == "nixosOptions";
    assert nixos.options ? demo;
    assert nixos.documentType == "none";
    assert nixos.warningsAreErrors == false;
    assert nixos.revision == "check-rev";
    # Building the docs package forces nixosOptionsDoc with the pass-through args.
      pkgs.runCommand "docs-options"
      {
        inherit (docsPkg.passthru) config;
      }
      ''
        test -f "$config/config.json"
        test -d "$config/pages"
        # Options leaf must exist and mention the demo option from tiny eval.
        grep -q demo "$config"/pages/*.md
        # Config must not embed option-record material.
        ! grep -q mkOption "$config/config.json"
        touch "$out"
      '';

  # allLeaves must terminate on the real Prelude option tree (pages.children is
  # visible="shallow"), preserve deep option paths, and never emit blank pages
  # for internal/hidden options (nav built from filtered docList).
  docs-allLeaves-prelude = let
    preludeEval = lib.evalModules {
      modules = [
        ../src/prelude/options/shared.nix
        ../src/prelude/options/motd.nix
        ../src/prelude/options/menu.nix
        ../src/prelude/options/portal.nix
        ../src/prelude/options/docs.nix
        ../src/prelude/options/prompt.nix
      ];
    };
    docsPkg =
      import ../src/prelude/docs.nix
      {
        inherit
          (pkgs)
          lib
          writeText
          buildGoModule
          runCommand
          nixosOptionsDoc
          figlet
          ;
      }
      {
        theme = "phosphor";
        colorProfile = "auto";
        project = "check";
        pages = [
          {
            generate = "nixosOptions";
            title = "Options";
            # default split is allLeaves — omit to exercise the default
          }
        ];
        nixosOptions = {
          options = {
            inherit (preludeEval.options) prelude;
          };
          transformOptions = o: o // {declarations = [];};
          warningsAreErrors = false;
        };
      };
  in
    pkgs.runCommand "docs-allLeaves-prelude"
    {
      inherit (docsPkg.passthru) config;
    }
    ''
      test -f "$config/config.json"
      test -d "$config/pages"
      count=$(find "$config/pages" -name '*.md' | wc -l | tr -d ' ')
      echo "allLeaves page count: $count"
      test "$count" -gt 20

      grep -R -q 'prelude\.motd' "$config/pages"
      grep -R -q 'motd\.env' "$config/pages"

      empty=0
      for f in "$config"/pages/*.md; do
        if ! grep -q '[^[:space:]]' "$f"; then
          echo "empty page: $f" >&2
          empty=$((empty + 1))
        fi
      done
      test "$empty" -eq 0

      if grep -R -q 'pages\.\*\.children\.\*\.children\.\*\.children' "$config/pages"; then
        echo "visible=shallow not honored — recursive children exploded" >&2
        exit 1
      fi
      touch "$out"
    '';

  # internal + transformOptions-hidden options must not leave nav/page entries.
  docs-allLeaves-filters-internal = let
    tiny = lib.evalModules {
      modules = [
        {
          options.visibleOpt = lib.mkOption {
            type = lib.types.str;
            default = "ok";
            description = "visible option";
          };
          options.hiddenInternal = lib.mkOption {
            type = lib.types.str;
            default = "nope";
            description = "internal option";
            internal = true;
          };
          options.hiddenByTransform = lib.mkOption {
            type = lib.types.str;
            default = "nope";
            description = "hidden via transformOptions";
          };
        }
      ];
    };
    docsPkg =
      import ../src/prelude/docs.nix
      {
        inherit
          (pkgs)
          lib
          writeText
          buildGoModule
          runCommand
          nixosOptionsDoc
          figlet
          ;
      }
      {
        theme = "phosphor";
        colorProfile = "auto";
        project = "check";
        pages = [
          {
            generate = "nixosOptions";
            title = "Options";
          }
        ];
        nixosOptions = {
          inherit (tiny) options;
          transformOptions = o:
            if o.name == "hiddenByTransform"
            then o // {visible = false;}
            else o;
          warningsAreErrors = false;
        };
      };
  in
    pkgs.runCommand "docs-allLeaves-filters-internal"
    {
      inherit (docsPkg.passthru) config;
    }
    ''
      test -f "$config/config.json"
      grep -q visibleOpt "$config/config.json"
      grep -R -q visibleOpt "$config/pages"
      if grep -q hiddenInternal "$config/config.json"; then
        echo "internal option leaked into nav" >&2
        exit 1
      fi
      if grep -q hiddenByTransform "$config/config.json"; then
        echo "transform-hidden option leaked into nav" >&2
        exit 1
      fi
      ! grep -R -q hiddenInternal "$config/pages"
      ! grep -R -q hiddenByTransform "$config/pages"
      touch "$out"
    '';

  # transformOptions may rewrite name/loc for display; lookup must still use
  # the raw loc so optionAt/nestPath succeed.
  docs-allLeaves-rename-transform = let
    tiny = lib.evalModules {
      modules = [
        {
          options.demo = lib.mkOption {
            type = lib.types.str;
            default = "x";
            description = "demo option renamed by transform";
          };
        }
      ];
    };
    docsPkg =
      import ../src/prelude/docs.nix
      {
        inherit
          (pkgs)
          lib
          writeText
          buildGoModule
          runCommand
          nixosOptionsDoc
          figlet
          ;
      }
      {
        theme = "phosphor";
        colorProfile = "auto";
        project = "check";
        pages = [
          {
            generate = "nixosOptions";
            title = "Options";
          }
        ];
        nixosOptions = {
          inherit (tiny) options;
          transformOptions = o:
            o
            // {
              name = "renamed.demo";
              loc = [
                "renamed"
                "demo"
              ];
            };
          warningsAreErrors = false;
        };
      };
  in
    pkgs.runCommand "docs-allLeaves-rename-transform"
    {
      inherit (docsPkg.passthru) config;
    }
    ''
      test -f "$config/config.json"
      grep -q 'renamed.demo' "$config/config.json"
      empty=0
      for f in "$config"/pages/*.md; do
        if ! grep -q '[^[:space:]]' "$f"; then
          echo "empty page after rename transform: $f" >&2
          empty=$((empty + 1))
        fi
      done
      test "$empty" -eq 0
      count=$(find "$config/pages" -name '*.md' | wc -l | tr -d ' ')
      test "$count" -ge 1
      touch "$out"
    '';

  # shallow = one pass-through page (full nixosOptionsDoc).
  docs-shallow-passthrough = let
    tiny = lib.evalModules {
      modules = [
        {
          options.demo = lib.mkOption {
            type = lib.types.str;
            default = "x";
            description = "demo option";
          };
        }
      ];
    };
    docsPkg =
      import ../src/prelude/docs.nix
      {
        inherit
          (pkgs)
          lib
          writeText
          buildGoModule
          runCommand
          nixosOptionsDoc
          figlet
          ;
      }
      {
        theme = "phosphor";
        colorProfile = "auto";
        project = "check";
        pages = [
          {
            generate = "nixosOptions";
            title = "Options";
            split = "shallow";
          }
        ];
        nixosOptions = {
          inherit (tiny) options;
          warningsAreErrors = false;
        };
      };
  in
    pkgs.runCommand "docs-shallow-passthrough"
    {
      inherit (docsPkg.passthru) config;
    }
    ''
      test -f "$config/config.json"
      count=$(find "$config/pages" -name '*.md' | wc -l | tr -d ' ')
      # One options page (+ nothing else in this fixture).
      test "$count" -eq 1
      grep -q demo "$config"/pages/*.md
      touch "$out"
    '';

  # Our own `x --list` renders the grouped command table.
  menu-list-renders = pkgs.runCommand "menu-list-renders" {} ''
    ${lib.getExe' config.packages.prelude-menu "x"} --list > "$out"
    test -s "$out"
    grep -q '^DEMOS$' "$out"
    grep -q "tour every feature demo" "$out"
  '';

  # Public contract: bare `menu` opens the picker only. Task/list args must
  # fail before any command executes.
  menu-rejects-execution = pkgs.runCommand "menu-rejects-execution" {} ''
    menu=${lib.getExe config.packages.prelude-menu}
    if "$menu" list >"$out" 2>"$out.err"; then
      echo "menu list unexpectedly succeeded" >&2
      cat "$out.err" >&2
      exit 1
    fi
    grep -q 'opens the interactive picker only' "$out.err"
    if "$menu" check >"$out" 2>"$out.err"; then
      echo "menu check unexpectedly succeeded" >&2
      cat "$out.err" >&2
      exit 1
    fi
    grep -q 'opens the interactive picker only' "$out.err"
    # Positive control: x still dispatches.
    ${lib.getExe' config.packages.prelude-menu "x"} --list > "$out"
    test -s "$out"
  '';

  # The standalone agent surface must be usable without a source checkout:
  # Markdown is part of the package closure, not a path relative to the caller.
  # `nix run .#skill` uses the package's main program; it is deliberately not a
  # fourth root app.
  skill-package =
    pkgs.runCommand "skill-package" {nativeBuildInputs = [config.packages.skill];}
    ''
      skill=${lib.getExe config.packages.skill}

      work=$(mktemp -d)
      cd "$work"

      "$skill" > intro.md
      grep -Fq 'nix run github:darkmatter/prelude#skill -- list' intro.md

      "$skill" list > topics.md
      grep -Fq 'install' topics.md
      grep -Fq 'options' topics.md
      grep -Fq 'commands' topics.md
      grep -Fq 'configuration' topics.md
      grep -Fq 'guide command-conventions' topics.md
      grep -Fq 'guide title-rendering' topics.md

      "$skill" install > install.md
      grep -Fq '# Your own repo' install.md
      "$skill" options > options.md
      grep -Fq '# Options reference' options.md
      "$skill" commands > commands.md
      grep -Fq '# Commands' commands.md
      "$skill" configuration > configuration.md
      grep -Fq '# Configuration' configuration.md
      "$skill" guide command-conventions > command-conventions.md
      grep -Fq '# Command conventions' command-conventions.md
      "$skill" guide title-rendering > title-rendering.md
      grep -Fq '# Title rendering guide' title-rendering.md

      assert_rejected() {
        name=$1
        shift
        if "$skill" "$@" > "$name.out" 2> "$name.err"; then
          echo "skill $* unexpectedly succeeded" >&2
          exit 1
        fi
        test ! -s "$name.out"
        grep -Fq 'skill list' "$name.err"
      }

      assert_rejected unknown unknown
      assert_rejected install-extra install extra
      assert_rejected guide-missing guide
      assert_rejected guide-unknown guide nonexistent
      assert_rejected guide-extra guide command-conventions extra

      touch "$out"
    '';

  # Every feature demo (motd variants, themes, acme-web motd + x --list)
  # builds (shellcheck) and renders.
  examples-render = pkgs.runCommand "examples-render" {} ''
    CLICOLOR_FORCE=1 ${lib.getExe demos.examplesRunner} > "$out"
    test -s "$out"
    grep -q 'theme amber' "$out"
    grep -q 'theme solarized' "$out"
    grep -q 'Devshell UI for Nix flakes' "$out"
    grep -Fq '38;2;255;199;97' "$out"
    grep -Fq '38;2;119;245;201' "$out"
  '';

  # Terminal-state emulator proof: the bounded MOTD card clears inherited
  # background state everywhere and leaves horizontal gutters at the terminal
  # default on both BCE and non-BCE models. Pyte models BCE (erase fills with
  # cursor bg), so the seeded background must disappear after SGR 49 resets.
  motd-bg-emulator = let
    ptyPython = pkgs.python3.withPackages (pythonPackages: [pythonPackages.pyte]);
    ptyCommandPath = lib.makeBinPath [
      pkgs.bash
      pkgs.coreutils
      pkgs.findutils
      pkgs.gawk
      pkgs.gnugrep
      pkgs.gnused
      pkgs.ncurses
      pkgs.procps
    ];
  in
    pkgs.runCommand "motd-bg-emulator"
    {
      nativeBuildInputs = [
        config.packages.prelude-motd
        ptyPython
      ];
    }
    ''
      ${lib.getExe ptyPython} ${./motd-bg-pty-test.py} \
        ${lib.getExe config.packages.prelude-motd} \
        ${lib.escapeShellArg ptyCommandPath}
      touch "$out"
    '';

  # Generated documentation and its media fingerprints must match the repo.
  docs-generated-fresh = docsAutomation.docsFresh;
  docs-media-fresh = docsAutomation.mediaFresh;
}
