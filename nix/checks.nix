# Flake checks. Checks whose $out is a rendered preview use the `-render(s)`
# suffix — the previews utility (previews.nix) discovers them by name.
{ pkgs
, lib
, config
, demos
, docsAutomation
, previews
, flakePartsLib
, ...
}:
let
  preludeLib = import ./lib.nix { inherit lib; };
  internalLib = import ../src/prelude/lib.nix { inherit lib; };

  # The command-providing packages of the dogfood devshell (shell.nix).
  devshellCommandPackages = [
    config.packages.prelude
    pkgs.nix
    docsAutomation.sync
    docsAutomation.record
    previews
  ]
  # Consume the wrappers exposed by the evaluated Prelude package, just as a
  # downstream module can, instead of rebuilding knowledge from source config.
  ++ config.packages.menu.commandWrappers;

  # Assert that every advertised canonical invocation starts with an executable
  # provided by the devshell. Group selectors (`go:test`) are menu identity and
  # intentionally do not exist on PATH; their invocation (`go test`) does.
  mkRunnableCheck =
    checkName: surface: invocations:
    let
      executableForLine =
        line: builtins.head (lib.filter (token: token != "") (lib.splitString " " line));
      invocationExecutables =
        invocation: map executableForLine (lib.filter (line: line != "") (lib.splitString "\n" invocation));
      executables = lib.unique (lib.concatMap invocationExecutables invocations);
    in
    pkgs.runCommand checkName { nativeBuildInputs = devshellCommandPackages; } ''
      for cmd in ${lib.concatMapStringsSep " " lib.escapeShellArg executables}; do
        command -v "$cmd" >/dev/null 2>&1 || {
          echo "${surface} advertises canonical executable '$cmd' but no devshell package provides it" >&2
          exit 1
        }
      done
      touch "$out"
    '';
in
{
  # Building the module-produced packages runs shellcheck / go vet on the
  # generated artifacts.
  motd-default = config.packages.motd;
  title-default = config.packages.title;
  menu-default = config.packages.menu;
  prelude-default =
    pkgs.runCommand "prelude-default"
      {
        nativeBuildInputs = [
          config.packages.prelude
          pkgs.shellcheck
        ];
      }
      ''
                command -v motd >/dev/null
                command -v menu >/dev/null
                command -v docs >/dev/null
                command -v starship >/dev/null
                command -v blesh-share >/dev/null
                test -f ${config.packages.prelude}/share/blesh/ble.sh
                test -f ${config.packages.prelude}/share/prelude/init.bash
                test -f ${config.packages.prelude}/share/prelude/shell/init.bash
                test -f ${config.packages.prelude}/share/prelude/shell/bash-init.bash
                test -f ${config.packages.prelude}/share/prelude/shell/status.bash
                test -f ${config.packages.prelude}/share/prelude/shell/completion.bash
                test -f ${config.packages.prelude}/share/prelude/shell/status-cap.bash
                test -f ${config.packages.prelude}/share/prelude/shell/catalogue.bash
                test -f ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash
                test -f ${config.packages.prelude}/nix-support/setup-hook
                grep -Fq 'prelude-init()' ${config.packages.prelude}/nix-support/setup-hook
                grep -Fq '. ${config.packages.prelude.shellInit}' ${config.packages.prelude}/nix-support/setup-hook
                grep -Fq '_PRELUDE_BLESH=${pkgs.blesh}/share/blesh/ble.sh' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq '_PRELUDE_STARSHIP=${lib.getExe pkgs.starship}' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq '_PRELUDE_STARSHIP_STATUS_ENABLED=1' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq 'bleopt color_scheme=prelude' ${config.packages.prelude}/share/prelude/shell/bash-init.bash
                grep -Fq 'function ble/contrib/scheme:prelude/initialize' ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash
                grep -Fq "ble-face -d prelude_status_cap" ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash
                test "$(grep -c '^  ble-face -[sd] ' ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash)" -eq 75
                ! grep -Fq '%prelude_' ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash
                ! grep -Eq '#[[:xdigit:]]{6}[[:alnum:]_]' ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash
                ! grep -Fq 'right_format' ${config.packages.prompt}
                grep -Fq '[❯](bold fg:accent2) ' ${config.packages.prompt}
                ! grep -Fq 'Type a command' ${config.packages.prompt}
                grep -Fq '_PRELUDE_PROMPT_PROJECT=' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq '_PRELUDE_PROMPT_NAVIGATION=' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq '_PRELUDE_PROMPT_NAVIGATION_RENDERED=' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq '_PRELUDE_PROMPT_STATUS_HINT=' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq '_PRELUDE_PROMPT_STATUS_HINT_RENDERED=' ${config.packages.prelude}/share/prelude/init.bash
                grep -Fq "bleopt prompt_status_line='\\q{prelude/status}'" ${config.packages.prelude}/share/prelude/shell/bash-init.bash
                grep -Fq "blehook PRECMD!='prelude/status/update'" ${config.packages.prelude}/share/prelude/shell/bash-init.bash
                (
                  COLUMNS=200
                  _PRELUDE_STARSHIP_STATUS_ENABLED=1
                  _PRELUDE_PROMPT_NAVIGATION='[?] motd  [m] menu  [d] docs'
                  _PRELUDE_PROMPT_NAVIGATION_RENDERED='\g{fg=#555555,bg=#101010}[\g{bold,fg=#00ff00,bg=#101010}?\g{fg=#555555,bg=#101010}]\g{fg=#777777,bg=#101010} motd  \g{fg=#555555,bg=#101010}[\g{bold,fg=#00ff00,bg=#101010}m\g{fg=#555555,bg=#101010}]\g{fg=#777777,bg=#101010} menu  \g{fg=#555555,bg=#101010}[\g{bold,fg=#00ff00,bg=#101010}d\g{fg=#555555,bg=#101010}]\g{fg=#777777,bg=#101010} docs'
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
                  source ${config.packages.prelude}/share/prelude/shell/catalogue.bash
                  source ${config.packages.prelude}/share/prelude/shell/status.bash
                  _prelude_status_revision=42
                  prelude/status/update
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_literal" | grep -F 'Welcome demo'
                  printf '%s' "$_prelude_fake_literal" | grep -F 'bare x'
                  printf '%s' "$_prelude_fake_literal" | grep -F 'x --list'
                  printf '%s' "$_prelude_fake_literal" | grep -F 'Tab'
                  printf '%s' "$_prelude_fake_line" | grep -F "$_PRELUDE_PROMPT_STATUS_HINT_RENDERED"
                  ! printf '%s' "$_prelude_fake_line" | grep -F '\g{none}'
                  test "$_prelude_fake_process_count" -eq 1
                  test "$_prelude_fake_processed" = "   $_PRELUDE_PROMPT_STATUS_HINT_RENDERED"
                  test "$_prelude_fake_hash_count" -eq 3
                  printf '%s' "$_prelude_fake_hashes" | grep -F '<$_ble_edit_str>'
                  printf '%s' "$_prelude_fake_hashes" | grep -F '<$_prelude_status_revision>'
                  printf '%s' "$_prelude_fake_hashes" | grep -F '<$_prelude_status_health_record>'
                  COLUMNS=40
                  _ble_edit_str=
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F 'Welcome'
                  COLUMNS=200
                  COLUMNS=25
                  _ble_edit_str=
                  _prelude_fake_process_count=0
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_literal" | grep -F 'Welcome'
                  test "$_prelude_fake_process_count" -eq 1
                  test "$_prelude_fake_processed" = "   $_PRELUDE_PROMPT_STATUS_HINT_RENDERED"
                  ! printf '%s' "$_prelude_fake_line" | grep -F 'motd'
                  COLUMNS=200

                  _ble_edit_str='x'
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F '`x <cmd>` for hints'
                  printf '%s' "$_prelude_fake_line" | grep -F 'cycle'
                  printf '%s' "$_prelude_fake_line" | grep -F 'navigate'

                  _ble_edit_str='x '
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F '`x <cmd>` for hints'
                  printf '%s' "$_prelude_fake_line" | grep -F 'cycle'
                  printf '%s' "$_prelude_fake_line" | grep -F 'navigate'

                  _ble_edit_str='x build'
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F 'build a flake output'
                  printf '%s' "$_prelude_fake_line" | grep -F 'x build'
                  printf '%s' "$_prelude_fake_line" | grep -F 'bare x then Tab'

                  _ble_edit_str='x build '
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F 'argument <empty>'
                  printf '%s' "$_prelude_fake_line" | grep -F 'optional'
                  printf '%s' "$_prelude_fake_line" | grep -F 'candidates: .#motd'

                  _ble_edit_str='x build .#m'
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F 'argument .#m'
                  printf '%s' "$_prelude_fake_line" | grep -F 'optional'
                  printf '%s' "$_prelude_fake_line" | grep -F 'flake output to build'
                  printf '%s' "$_prelude_fake_line" | grep -F 'candidates: .#motd'

                  _ble_edit_str="x '"
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F 'Welcome demo'

                  _ble_edit_str='x unknown'
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_line" | grep -F 'Welcome demo'

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
                  ble/prompt/backslash:prelude/status
                  printf '%s' "$_prelude_fake_literal" | grep -F '\g{fg=#ff0000}unsafe'
                  test "$_prelude_fake_process_count" -eq 1
                  test "$_prelude_fake_processed" = "   $_PRELUDE_PROMPT_STATUS_HINT_RENDERED"
                  ! printf '%s' "$_prelude_fake_processed" | grep -F 'unsafe'
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
                  test "$_prelude_fake_process_count" -eq 1
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
                  source ${config.packages.prelude}/share/prelude/shell/status-cap.bash
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
                  test "$_prelude_cap_output" = '<cap>▄▄▄▄<reset>'
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
                  source ${config.packages.prelude}/share/prelude/shell/status-cap.bash
                  ! prelude/status/cap/install >/dev/null 2>&1
                  test "''${#_ble_canvas_panel_class[@]}" -eq 5
                  test "''${_ble_canvas_panel_class[4]}" = unexpected
                  test "$_ble_prompt_status_panel" -eq 4
                  test "$_ble_canvas_panel_vfill" -eq 4
                )
                ${pkgs.bash}/bin/bash -n ${config.packages.prelude}/share/prelude/init.bash
                for source in ${config.packages.prelude}/share/prelude/shell/*.bash; do
                  ${pkgs.bash}/bin/bash -n "$source"
                done
                ${pkgs.bash}/bin/bash -n ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash
                shellcheck -x ${config.packages.prelude}/share/prelude/init.bash
                shellcheck -x ${config.packages.prelude}/share/prelude/shell/init.bash
                shellcheck -x -e SC1091,SC2154 ${config.packages.prelude}/share/prelude/shell/bash-init.bash
                shellcheck -x -e SC2016,SC2154 ${config.packages.prelude}/share/prelude/shell/status.bash
                shellcheck -x -e SC2154 ${config.packages.prelude}/share/prelude/shell/completion.bash
                shellcheck -e SC2154 ${config.packages.prelude}/share/prelude/shell/contrib/scheme/prelude.bash
                touch "$out"
      '';

  title-previews = pkgs.runCommand "title-previews" { } ''
    ${lib.getExe config.packages.title-previews} "choose me" > "$out"
    test "$(grep -c '^===== .* =====$' "$out")" -eq 25
    grep -q '^===== 3d-ascii =====$' "$out"
    grep -q '^===== calvin-s =====$' "$out"
    grep -q '^===== roman =====$' "$out"
    grep -q '^===== univers =====$' "$out"
    test "$(wc -l < "$out")" -gt 50
  '';

  title-generates =
    let
      # JSON, not Nix: nix-instantiate cannot write to /nix/var/nix/profiles
      # inside the build sandbox, so the title tool's Nix-recipe path is
      # unusable here. The tool accepts JSON recipes directly.
      recipe = pkgs.writeText "title.json" ''{"text":"prelude","font":"calvin-s"}'';
    in
    pkgs.runCommand "title-generates" { } ''
      ${lib.getExe config.packages.title} --recipe ${recipe} --output "$out"
      grep -q '┌─┐' "$out"
    '';

  # fromPkg is a small adapter over mkCommand: package selection is positional,
  # while program/arguments and presentation metadata stay composable extras.
  from-pkg =
    let
      command = preludeLib.fromPkg pkgs.nixfmt {
        arguments = [ "." ];
        description = "format Nix sources";
        key = "f";
      };
    in
    assert command.description == "format Nix sources";
    assert command.key == "f";
    assert command.invocation == "nixfmt .";
    assert lib.hasPrefix (lib.getExe pkgs.nixfmt) command.exec;
    assert command.runtimePackages == [ pkgs.nixfmt ];
    pkgs.runCommand "from-pkg" { } "touch $out";

  # Prelude owns navigation commands. `menu` is always advertised on the MOTD
  # (bare, no `x` prefix); docs stays menu-only. Project Getting Started rows
  # remain focused on explicitly selected lifecycle commands.
  prelude-command-defaults =
    assert lib.all (name: lib.elem name config.packages.menu.commandNames) [
      "menu"
      "docs"
    ];
    assert lib.elem "menu" config.packages.motd.commandNames;
    assert lib.elem "menu" config.packages.motd.commandInvocations;
    assert !lib.elem "x menu" config.packages.motd.commandInvocations;
    assert !lib.elem "docs" config.packages.motd.commandNames;
    pkgs.runCommand "prelude-command-defaults" { nativeBuildInputs = [ config.packages.menu ]; } ''
      command -v x >/dev/null
      command -v menu >/dev/null
      command -v docs >/dev/null
      ! command -v help >/dev/null
      touch "$out"
    '';

  # Complete command keys stay public while the first colon derives group/label
  # presentation. Prelude stays first and configured groups follow in order.
  command-ordering =
    let
      plib = import ../src/prelude/lib.nix { inherit lib; };
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
              menu = { };
              dev = { };
              "docs:sync" = { };
              "docs:record" = { };
              "demos:menu".exec = "nix run .#example-menu";
            };
          }
          {
            prelude.commands."docs:record".description = "merged";
          }
        ];
      };
      normalized = plib.normalizeCommandGroups evaluated.config.prelude.sort.groups evaluated.config.prelude.commands;
      actual = map
        (group: {
          inherit (group) title;
          commands = map
            (command: {
              inherit (command)
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
              name = "menu";
              label = "menu";
              run = "menu";
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
    pkgs.runCommand "command-ordering" { } "touch $out";

  # Header options share one nested namespace for module and direct consumers.
  motd-header-options =
    let
      evaluated = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/motd.nix
          {
            prelude.motd = {
              title = {
                text = pkgs.writeText "test-title.txt" "TEST TITLE\n";
                align = "center";
                style = "bracketed";
              };
              padding = {
                x = 2;
                top = 2;
              };
              windowBackground = {
                blend = 0.15;
              };
              header = {
                tagline = {
                  text = "test-tagline";
                  subtitle = "test-subtitle";
                  layout = "inline";
                  align = "center";
                };
                statusHint.layout = "inline";
                status.shell = {
                  order = 100;
                  label = "nix develop";
                  status = "ready";
                };
                status.cache = {
                  order = 200;
                  label = "cache";
                  check = "false";
                  fail = "stale";
                  failLevel = "warning";
                };
              };
              links = [
                {
                  label = "Prelude on GitHub";
                  url = "https://github.com/darkmatter/prelude";
                }
              ];
            };
          }
        ];
      };
      title = evaluated.config.prelude.motd.title;
      header = evaluated.config.prelude.motd.header;
      padding = evaluated.config.prelude.motd.padding;
      links = evaluated.config.prelude.motd.links;
      windowBackground = evaluated.config.prelude.motd.windowBackground;
      shellStatus = header.status.shell;
      exposesShortcutOption = evaluated.options.prelude.motd ? shortcuts;
    in
    assert builtins.readFile title.text == "TEST TITLE\n";
    assert title.align == "center";
    assert title.style == "bracketed";
    assert header.tagline.text == "test-tagline";
    assert header.tagline.subtitle == "test-subtitle";
    assert header.tagline.layout == "inline";
    assert header.tagline.align == "center";
    assert header.statusHint.layout == "inline";
    assert shellStatus.label == "nix develop";
    assert shellStatus.status == "ready";
    assert shellStatus.failLevel == "error";
    assert header.status.cache.failLevel == "warning";
    assert header.status.cache.async;
    assert
    links == [
      {
        label = "Prelude on GitHub";
        url = "https://github.com/darkmatter/prelude";
      }
    ];
    assert padding.x == 2;
    assert padding.y == 2;
    assert padding.top == 2;
    assert padding.left == null;
    assert padding.right == null;
    assert windowBackground.blend == 0.15;
    assert !exposesShortcutOption;

    pkgs.runCommand "motd-header-options" { } "touch $out";

  # Core navigation shortcuts are synthesized from component availability;
  # consumers cannot remove or advertise commands that are disabled.
  component-shortcuts =
    let
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
    assert
    all == [
      {
        command = "motd";
        alias = "?";
      }
      {
        command = "menu";
        alias = "m";
      }
      {
        command = "docs";
        alias = "d";
      }
    ];
    assert
    menuOnly == [
      {
        command = "menu";
        alias = "m";
      }
    ];
    pkgs.runCommand "component-shortcuts" { } "touch $out";

  # prelude.lib.mdSplit → { title = "README"; text; children = [preamble, H2…] }.
  # docs.nix renames first child to project + rootReadme when text matches.
  mdSplit-pages =
    let
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
    assert
    titles == [
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
    assert
    thinTitles == [
      "Thin"
      "Alpha"
    ];
    assert lib.hasInfix "alpha body" (builtins.readFile (builtins.elemAt thin.children 1).text);
    pkgs.runCommand "mdSplit-pages" { } "touch $out";

  # docs.nix nav: README → <project> → first original H2 … + FIGlet flag.
  mdSplit-readme-nav =
    let
      docsPkg =
        import ../src/prelude/docs.nix
          {
            inherit (pkgs)
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
              options = { };
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
    pkgs.runCommand "mdSplit-readme-nav" { } "touch $out";
  prompt-shadow-palette =
    let
      mkPrompt =
        promptConfig:
        (import ../src/prelude/prompt.nix {
          inherit (pkgs) lib formats;
        })
          promptConfig;
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
      themePrompts = lib.mapAttrs (theme: _: mkPrompt { inherit theme; }) themeCases;
      themeChecks = lib.concatStringsSep "\n" (
        lib.mapAttrsToList
          (
            theme: shadow: ''grep -Fq 'shadow = "${shadow}"' ${themePrompts.${theme}}''
          )
          themeCases
      );
      overridden = mkPrompt {
        theme = "apathy";
        palette.bg = "#6496c8";
      };
      black = mkPrompt {
        theme = "apathy";
        palette.bg = "#000000";
      };
      literalWindow = mkPrompt {
        theme = "apathy";
        windowBackgroundContext = {
          set = true;
          base = "#202020";
        };
      };
      dynamicWindow = mkPrompt {
        theme = "apathy";
        windowBackgroundContext = {
          set = true;
          base = null;
        };
      };
      injectedBackdrop = mkPrompt {
        theme = "apathy";
        backdropPalette = internalLib.resolveBackdropPalette "apathy" { } literalWindowContext;
      };
      shortHex = mkPrompt {
        theme = "apathy";
        palette.bg = "#abc";
      };
      indexed = mkPrompt {
        theme = "apathy";
        palette.bg = 212;
      };
      packed = mkPrompt {
        theme = "apathy";
        palette.bg = 660510;
      };
      disabledWindowContext = internalLib.resolveWindowBackgroundContext false "#202020";
      noWindowContext = internalLib.resolveWindowBackgroundContext true null;
      transparentWindowContext = internalLib.resolveWindowBackgroundContext true false;
      themeWindowContext = internalLib.resolveWindowBackgroundContext true true;
      literalWindowContext = internalLib.resolveWindowBackgroundContext true "#202020";
      relativeWindowContext = internalLib.resolveWindowBackgroundContext true { relative = -0.05; };
      blendWindowContext = internalLib.resolveWindowBackgroundContext true { blend = 0.4; };
      mkShell =
        shadow: windowBackgroundSet:
        (import ../src/prelude/shell-init.nix {
          inherit (pkgs)
            lib
            writeText
            runCommand
            starship
            blesh
            bash-completion
            stdenv
            ;
        }) {
          palette = internalLib.resolvePalette "apathy" { };
          inherit shadow windowBackgroundSet;
        };
      ownedShell = mkShell "#1e1e1e" true;
      fallbackShell = mkShell "#0d0a12" false;
    in
    assert !disabledWindowContext.set;
    assert disabledWindowContext.base == null;
    assert relativeWindowContext.set;
    assert relativeWindowContext.base == null;
    assert !noWindowContext.set;
    assert noWindowContext.base == null;
    assert !transparentWindowContext.set;
    assert transparentWindowContext.base == null;
    assert themeWindowContext.set;
    assert themeWindowContext.base == null;
    assert literalWindowContext.set;
    assert literalWindowContext.base == "#202020";
    assert blendWindowContext.set;
    assert blendWindowContext.base == null;
    pkgs.runCommand "prompt-shadow-palette" { } ''
      ${themeChecks}
      grep -Fq 'shadow = "#6798c9"' ${overridden}
      grep -Fq 'shadow = "#060606"' ${black}
      grep -Fq 'shadow = "#252525"' ${literalWindow}
      grep -Fq 'shadow = "#141118"' ${dynamicWindow}
      grep -Fq 'shadow = "#252525"' ${injectedBackdrop}
      grep -Fq 'shadow = "#acbccd"' ${shortHex}
      grep -Fq 'shadow = "#ff8ad8"' ${indexed}
      grep -Fq 'shadow = "#101923"' ${packed}
      grep -Fq '_PRELUDE_WINDOW_BACKGROUND_SET=1' ${ownedShell.init}
      grep -Fq '_PRELUDE_WINDOW_BACKGROUND_SET=0' ${fallbackShell.init}
      cat > "$TMPDIR/probe-cap-face.bash" <<'EOF'
      set -euo pipefail
      scheme=$1
      shadow=$2
      background=$3
      faces=$4
      ble-import() { :; }
      ble/contrib/scheme:default/initialize() { :; }
      cap_defined=
      ble-face() {
        if [[ $2 == prelude_status_cap ]]; then
          case $1 in
            -d) cap_defined=1 ;;
            -s) [[ $cap_defined ]] || exit 1 ;;
          esac
        fi
        printf '%s\n' "$*" >> "$faces"
      }
      source "$scheme"
      ble/contrib/scheme:prelude/initialize
      grep -Fx -- "-d prelude_status_cap fg=#1b1629,bg=$background" "$faces"
      grep -Fx -- "-s prelude_status_cap fg=#1b1629,bg=$background" "$faces"
      : > "$faces"
      cap_defined=
      _prelude_window_background_set=1
      ble/contrib/scheme:prelude/initialize
      grep -Fx -- "-s prelude_status_cap fg=#1b1629,bg=$shadow" "$faces"
      : > "$faces"
      prelude/status/cap/refresh-face 1
      grep -Fx -- "-s prelude_status_cap fg=#1b1629,bg=$shadow" "$faces"
      : > "$faces"
      prelude/status/cap/refresh-face 0
      grep -Fx -- "-s prelude_status_cap fg=#1b1629,bg=$background" "$faces"
      EOF
      ${lib.getExe pkgs.bash} "$TMPDIR/probe-cap-face.bash" \
        ${ownedShell.runtime}/contrib/scheme/prelude.bash "#1e1e1e" "#0e0b13" "$TMPDIR/owned-faces"
      ${lib.getExe pkgs.bash} "$TMPDIR/probe-cap-face.bash" \
        ${fallbackShell.runtime}/contrib/scheme/prelude.bash "#0d0a12" "#0e0b13" "$TMPDIR/fallback-faces"

      mkdir -p "$TMPDIR/init-runtime"
      cp ${ownedShell.runtime}/init.bash "$TMPDIR/init-runtime/init.bash"
      : > "$TMPDIR/init-runtime/catalogue.bash"
      cat > "$TMPDIR/init-runtime/bash-init.bash" <<'EOF'
      prelude/status/cap/refresh-face() {
        printf 'refresh:%s\n' "$1" > "$PRELUDE_FACE_RESULT"
      }
      _prelude_init_show_motd
      printf '%s\n' "$_prelude_window_background_set" > "$PRELUDE_OWNERSHIP_RESULT"
      EOF
      sed \
        -e "s|^_PRELUDE_SHELL_RUNTIME=.*|_PRELUDE_SHELL_RUNTIME=$TMPDIR/init-runtime|" \
        -e '/^_PRELUDE_MOTD=/d' \
        -e 's|^\. /nix/store/.*/init\.bash$|. "$_PRELUDE_SHELL_RUNTIME/init.bash"|' \
        ${ownedShell.init} > "$TMPDIR/owned-init.bash"
      sed \
        -e "s|^_PRELUDE_SHELL_RUNTIME=.*|_PRELUDE_SHELL_RUNTIME=$TMPDIR/init-runtime|" \
        -e '/^_PRELUDE_MOTD=/d' \
        -e 's|^\. /nix/store/.*/init\.bash$|. "$_PRELUDE_SHELL_RUNTIME/init.bash"|' \
        ${fallbackShell.init} > "$TMPDIR/fallback-init.bash"
      grep -Fqx "_PRELUDE_SHELL_RUNTIME=$TMPDIR/init-runtime" "$TMPDIR/owned-init.bash"
      grep -Fqx '. "$_PRELUDE_SHELL_RUNTIME/init.bash"' "$TMPDIR/owned-init.bash"
      grep -Fqx "_PRELUDE_SHELL_RUNTIME=$TMPDIR/init-runtime" "$TMPDIR/fallback-init.bash"
      grep -Fqx '. "$_PRELUDE_SHELL_RUNTIME/init.bash"' "$TMPDIR/fallback-init.bash"
      cat > "$TMPDIR/motd" <<EOF
      #!${lib.getExe pkgs.bash}
      touch "$TMPDIR/motd-ran"
      EOF
      chmod +x "$TMPDIR/motd"
      cat > "$TMPDIR/failing-motd" <<EOF
      #!${lib.getExe pkgs.bash}
      exit 1
      EOF
      chmod +x "$TMPDIR/failing-motd"
      env \
        _PRELUDE_MOTD="$TMPDIR/motd" \
        PRELUDE_FACE_RESULT="$TMPDIR/face-result" \
        PRELUDE_OWNERSHIP_RESULT="$TMPDIR/ownership-result" \
        PRELUDE_POST_INIT_RESULT="$TMPDIR/post-init-result" \
        ${lib.getExe pkgs.bash} --noprofile --norc -ic '. "$1"; printf "%s\n" "$_prelude_window_background_set" > "$PRELUDE_POST_INIT_RESULT"' _ "$TMPDIR/owned-init.bash" \
        > "$TMPDIR/init-normal-output" 2>&1
      test -e "$TMPDIR/motd-ran"
      test "$(cat "$TMPDIR/face-result")" = refresh:1
      test "$(cat "$TMPDIR/ownership-result")" = 1
      test "$(cat "$TMPDIR/post-init-result")" = 1
      rm -f "$TMPDIR/motd-ran" "$TMPDIR/face-result" "$TMPDIR/ownership-result" "$TMPDIR/post-init-result"
      env \
        _PRELUDE_MOTD="$TMPDIR/motd" \
        PRELUDE_FACE_RESULT="$TMPDIR/face-result" \
        PRELUDE_OWNERSHIP_RESULT="$TMPDIR/ownership-result" \
        PRELUDE_POST_INIT_RESULT="$TMPDIR/post-init-result" \
        ${lib.getExe pkgs.bash} --noprofile --norc -ic '. "$1"; printf "%s\n" "$_prelude_window_background_set" > "$PRELUDE_POST_INIT_RESULT"' _ "$TMPDIR/fallback-init.bash" \
        > "$TMPDIR/init-fallback-output" 2>&1
      test -e "$TMPDIR/motd-ran"
      test "$(cat "$TMPDIR/face-result")" = refresh:0
      test "$(cat "$TMPDIR/ownership-result")" = 0
      test "$(cat "$TMPDIR/post-init-result")" = 0
      rm -f "$TMPDIR/motd-ran" "$TMPDIR/face-result" "$TMPDIR/ownership-result" "$TMPDIR/post-init-result"
      env \
        PRELUDE_INIT_QUIET=1 \
        _PRELUDE_MOTD="$TMPDIR/motd" \
        PRELUDE_FACE_RESULT="$TMPDIR/face-result" \
        PRELUDE_OWNERSHIP_RESULT="$TMPDIR/ownership-result" \
        PRELUDE_POST_INIT_RESULT="$TMPDIR/post-init-result" \
        ${lib.getExe pkgs.bash} --noprofile --norc -ic '. "$1"; printf "%s\n" "$_prelude_window_background_set" > "$PRELUDE_POST_INIT_RESULT"' _ "$TMPDIR/owned-init.bash" \
        > "$TMPDIR/init-quiet-output" 2>&1
      test ! -e "$TMPDIR/motd-ran"
      test "$(cat "$TMPDIR/face-result")" = refresh:0
      test "$(cat "$TMPDIR/ownership-result")" = 0
      test "$(cat "$TMPDIR/post-init-result")" = 0
      rm -f "$TMPDIR/face-result" "$TMPDIR/ownership-result" "$TMPDIR/post-init-result"
      env \
        _PRELUDE_MOTD="$TMPDIR/failing-motd" \
        PRELUDE_FACE_RESULT="$TMPDIR/face-result" \
        PRELUDE_OWNERSHIP_RESULT="$TMPDIR/ownership-result" \
        PRELUDE_POST_INIT_RESULT="$TMPDIR/post-init-result" \
        ${lib.getExe pkgs.bash} --noprofile --norc -ic '. "$1"; printf "%s\n" "$_prelude_window_background_set" > "$PRELUDE_POST_INIT_RESULT"' _ "$TMPDIR/owned-init.bash" \
        > "$TMPDIR/init-failed-output" 2>&1
      test "$(cat "$TMPDIR/face-result")" = refresh:0
      test "$(cat "$TMPDIR/ownership-result")" = 0
      test "$(cat "$TMPDIR/post-init-result")" = 0
      grep -Fqx '_prelude_window_background_set=0' ${ownedShell.runtime}/bash-init.bash
      scheme_line=$(grep -n -Fx 'bleopt color_scheme=prelude' ${ownedShell.runtime}/bash-init.bash | cut -d: -f1)
      motd_line=$(grep -n -Fx '_prelude_init_show_motd' ${ownedShell.runtime}/bash-init.bash | cut -d: -f1)
      test "$scheme_line" -lt "$motd_line"
      ${lib.getExe pkgs.bash} -n ${ownedShell.runtime}/bash-init.bash
      ${lib.getExe pkgs.bash} -n ${ownedShell.runtime}/contrib/scheme/prelude.bash
      touch "$out"
    '';

  prompt-renders-shortcuts = pkgs.runCommand "prompt-renders-shortcuts" { } ''
    export NO_COLOR=1
    export HOME="$TMPDIR/home"
    export XDG_CACHE_HOME="$TMPDIR/cache"
    mkdir -p "$HOME" "$XDG_CACHE_HOME"
    export STARSHIP_CONFIG=${config.packages.prompt}
    export STARSHIP_SHELL=bash
    ${lib.getExe pkgs.starship} prompt --terminal-width 79 --status 0 > "$TMPDIR/normal"
    ${lib.getExe pkgs.starship} prompt --right --terminal-width 79 --status 0 > "$TMPDIR/status"

    # The generated prompt owns two blank rows before its two visible rows.
    test "$(od -An -t x1 -N 2 "$TMPDIR/normal" | tr -d '[:space:]')" = 0a0a
    sed -n '3p' "$TMPDIR/normal" | grep -F 'prelude'
    sed -n '4p' "$TMPDIR/normal" | grep -F '~/prelude'
    sed -n '4p' "$TMPDIR/normal" | grep -F '❯'
    ! grep -F 'motd' "$TMPDIR/normal"
    ! grep -F 'menu' "$TMPDIR/normal"
    ! grep -F 'docs' "$TMPDIR/normal"
    test ! -s "$TMPDIR/status"
    touch "$out"
  '';

  prompt-status-runtime =
    let
      statusPkg =
        (import ../src/prelude/prompt-status.nix {
          inherit (pkgs)
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
            palette = internalLib.resolvePalette "phosphor" { };
            projectName = "fixture";
            commandEntries = [ ];
            navigation = [ ];
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
  prompt-local-server-evaluation =
    let
      validLocalServer = {
        command = "dev";
        check = "true";
        ttl = "5m";
      };
      customConfigFile = ../nix/internal/title.txt;
      evalPrompt =
        localServer: configFile:
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
          inputs = { };
        };
      };
      fixtureSystem = pkgs.stdenv.hostPlatform.system;
      fixturePkgs = {
        inherit lib;
        # The test only reads the descriptor, so a store text path is enough.
        writeText = builtins.toFile;
        # Preserve prompt-status.nix's observable passthru contract directly.
        buildGoModule = args: args.passthru;
        symlinkJoin =
          args:
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
        localFlake = { };
        flake-parts-lib = flakePartsLib;
      };
      evalFixture =
        localServer:
        let
          evaluated = flakePartsLib.evalFlakeModule { inputs = fixtureInputs; } {
            systems = [ fixtureSystem ];
            imports = [ fixtureModule ];
            prelude = {
              project = "fixture";
              prompt = {
                enable = true;
                inherit localServer;
              };
            };
            perSystem = { ... }: {
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
        builtins.deepSeq evaluated.config.allSystems.${fixtureSystem}.packages.prelude.promptStatusPkg
          evaluated.config.allSystems.${fixtureSystem}.packages.prelude.promptStatusPkg;
      valid = evalPrompt validLocalServer null;
      invalidTtl = evalPrompt (validLocalServer // { ttl = "0m"; }) null;
      invalidOverflowTtl = evalPrompt (validLocalServer // { ttl = "9223372036854775807h"; }) null;
      invalidCheck = evalPrompt (validLocalServer // { check = "  "; }) null;
      custom = evalPrompt validLocalServer customConfigFile;
      perSystemValid = evalFixture (validLocalServer // { command = commandKey; });
      perSystemUnknown = builtins.tryEval (evalFixture (validLocalServer // { command = "missing"; }));
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
    pkgs.runCommand "prompt-local-server-evaluation" { nativeBuildInputs = [ pkgs.jq ]; } ''
      ${lib.getExe pkgs.jq} -e --arg start ${lib.escapeShellArg start} \
        '.start == $start' ${statusConfig} >/dev/null
      touch "$out"
    '';

  # The MOTD advertises x aliases for project commands (plus bare `menu`);
  # the menu retains canonical underlying invocations for execution and
  # diagnostics.
  motd-commands-runnable =
    mkRunnableCheck "motd-commands-runnable" "motd"
      config.packages.motd.commandInvocations;

  menu-commands-runnable =
    mkRunnableCheck "menu-commands-runnable" "menu"
      config.packages.menu.commandInvocations;

  # Built-in navigation aliases must resolve on the same PATH as their labels.
  motd-shortcuts-runnable =
    assert
    config.packages.motd.shortcutAliases == [
      "?"
      "m"
      "d"
    ];
    mkRunnableCheck "motd-shortcuts-runnable" "built-in shortcuts" config.packages.motd.shortcutAliases;

  titles-command-renders =
    pkgs.runCommand "titles-command-renders"
      {
        nativeBuildInputs = [ config.packages.motd ];
      }
      ''
        prelude-title-previews prelude > "$out"
        test "$(grep -c '^===== .* =====$' "$out")" -eq 25
        grep -q '^===== 3d-ascii =====$' "$out"
        grep -q '^===== calvin-s =====$' "$out"
        test "$(wc -l < "$out")" -gt 50
      '';

  # Package-backed ungrouped aliases carry their runtime package and wrapper.
  package-command-bundled =
    assert lib.elem pkgs.nixfmt config.packages.menu.commandRuntimePackages;
    pkgs.runCommand "package-command-bundled"
      {
        nativeBuildInputs = [ config.packages.menu ];
      }
      ''
        command -v nixfmt >/dev/null
        command -v fmt >/dev/null
        touch "$out"
      '';

  colon-command-names-preserved =
    let
      internalPreludeLib = import ../src/prelude/lib.nix { inherit lib; };
      imported = internalPreludeLib.normalizeCommand "test:unit" {
        exec = "npm run test:unit";
      };
    in
    assert imported.name == "test:unit";
    assert imported.group == "test";
    assert imported.label == "unit";
    pkgs.runCommand "colon-command-names-preserved" { } "touch $out";

  duplicate-canonical-invocations-rejected =
    let
      internalPreludeLib = import ../src/prelude/lib.nix { inherit lib; };
      attempted = builtins.tryEval (
        builtins.deepSeq
          (internalPreludeLib.normalizeCommandEntries {
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
    pkgs.runCommand "duplicate-canonical-invocations-rejected" { } "touch $out";

  # Group prefixes are parsed into menu metadata and never become PATH names.
  # Canonical package invocations remain the native CLI syntax.
  grouped-commands-use-canonical-invocations =
    assert lib.elem "go:vet" config.packages.menu.commandNames;
    assert lib.elem "go vet -C src ./..." config.packages.menu.commandInvocations;
    assert lib.elem "x go:vet" config.packages.menu.xInvocations;
    assert !lib.elem "go:vet" config.packages.menu.commandWrapperNames;
    assert !lib.elem "go-vet" config.packages.menu.commandWrapperNames;
    pkgs.runCommand "grouped-commands-use-canonical-invocations"
      { nativeBuildInputs = [ config.packages.menu ]; }
      ''
        command -v go >/dev/null
        ! command -v go:vet >/dev/null
        ! command -v go-vet >/dev/null
        touch "$out"
      '';

  # Docs options accept nested nav nodes and full nixosOptionsDoc arg pass-through.
  docs-options =
    let
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
              { text = ../docs/welcome.md; }
              {
                title = "Guides";
                children = [
                  { text = ../docs/commands.md; }
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
            inherit (pkgs)
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
  docs-allLeaves-prelude =
    let
      preludeEval = lib.evalModules {
        modules = [
          ../src/prelude/options/shared.nix
          ../src/prelude/options/motd.nix
          ../src/prelude/options/menu.nix
          ../src/prelude/options/docs.nix
          ../src/prelude/options/prompt.nix
        ];
      };
      docsPkg =
        import ../src/prelude/docs.nix
          {
            inherit (pkgs)
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
              transformOptions = o: o // { declarations = [ ]; };
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
  docs-allLeaves-filters-internal =
    let
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
            inherit (pkgs)
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
              transformOptions = o: if o.name == "hiddenByTransform" then o // { visible = false; } else o;
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
  docs-allLeaves-rename-transform =
    let
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
            inherit (pkgs)
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
              transformOptions =
                o:
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
  docs-shallow-passthrough =
    let
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
            inherit (pkgs)
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
  menu-list-renders = pkgs.runCommand "menu-list-renders" { } ''
    ${lib.getExe' config.packages.menu "x"} --list > "$out"
    test -s "$out"
    grep -q '^DEMOS$' "$out"
    grep -q "tour every feature demo" "$out"
  '';

  # Public contract: bare `menu` opens the picker only. Task/list args must
  # fail before any command executes.
  menu-rejects-execution = pkgs.runCommand "menu-rejects-execution" { } ''
    menu=${lib.getExe config.packages.menu}
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
    ${lib.getExe' config.packages.menu "x"} --list > "$out"
    test -s "$out"
  '';

  # Every feature demo (motd variants, themes, acme-web motd + x --list)
  # builds (shellcheck) and renders.
  examples-render = pkgs.runCommand "examples-render" { } ''
    CLICOLOR_FORCE=1 ${lib.getExe demos.examplesRunner} > "$out"
    test -s "$out"
    grep -q 'theme amber' "$out"
    grep -q 'theme solarized' "$out"
    grep -q 'Devshell UI for Nix flakes' "$out"
    grep -Fq '38;2;255;199;97' "$out"
    grep -Fq '38;2;119;245;201' "$out"
  '';

  # Generated documentation and its media fingerprints must match the repo.
  docs-generated-fresh = docsAutomation.docsFresh;
  docs-media-fresh = docsAutomation.mediaFresh;
}
