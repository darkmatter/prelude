# flake-parts module: the prelude devshell UI suite.
#
#   prelude.motd  — devshell welcome banner
#   prelude.menu  — interactive command menu
#   prelude.docs  — hand-authored man-style project manual
#
# Shared config covers theme/palette and project identity. `groups` belongs to
# the menu; MOTD guidance and docs content are authored independently.
# Options are declared in ./options/{shared,motd,menu,docs}.nix.
#
#   outputs = { prelude, flake-parts, ... }@inputs:
#     flake-parts.lib.mkFlake { inherit inputs; } {
#       imports = [ prelude.flakeModules.prelude ];
#
#       prelude = {
#         theme = "phosphor";
#         project = "acme-web";
#         motd.header.tagline = "everything you need to build, test & ship";
#
#         groups.develop = {
#           order = 100;
#           tasks.dev = {
#             run = "pnpm dev";
#             description = "start the dev server with hot reload";
#             key = "d";
#           };
#         };
#
#         docs.sections.name.blocks = [
#           { type = "lead"; term = "acme-web"; text = "the product shell"; }
#         ];
#
#         motd.enable = true;
#         menu.enable = true;
#         docs.enable = true;
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

  mkMotd = import ./motd.nix;
  mkTitle = import ./title-generator.nix;
  mkTitlePreviews = import ./title-previews.nix;
  mkMenu = import ./menu.nix;
  mkDocs = import ./docs.nix;
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
  ];

  options.perSystem = flake-parts-lib.mkPerSystemOption ({ lib, ... }: {
    options.prelude.groups = lib.mkOption {
      type = lib.types.attrsOf optionTypes.groupType;
      default = { };
      description = "System-specific task groups, including package-backed tasks created with prelude.lib.mkTask.";
    };
  });

  config = {
    # Advertise the menu task by default; an explicit list replaces this.
    prelude.motd.commands = lib.mkIf cfg.menu.enable (lib.mkDefault [ "menu" ]);

    perSystem =
      { pkgs, config, ... }:
      let
        groups = lib.recursiveUpdate cfg.groups config.prelude.groups;
        deps = {
          inherit (pkgs)
            lib
            writeShellApplication
            writeText
            buildGoModule
            figlet
            jq
            nix
            ;
        };

        motdBin = mkMotd deps (generatorConfig cfg.motd // { inherit groups; });
        titlePkg = mkTitle deps;
        titlePreviewsPkg = mkTitlePreviews deps;

        motdPkg = pkgs.symlinkJoin {
          name = "motd";
          # Task-backed MOTD rows remain runnable when packages.motd is used
          # directly by carrying the menu and its generated task wrappers.
          paths = [
            motdBin
            titlePkg
          ]
          ++ lib.optional cfg.menu.enable menuPkg;
          passthru = {
            commandNames = cfg.motd.commands;
            taskWrappers = lib.optionals cfg.menu.enable menuPkg.taskWrappers;
          };
          meta = {
            inherit (motdBin.meta) description;
            mainProgram = "motd";
          };
        };

        menuBin = mkMenu deps (generatorConfig cfg.menu // { inherit groups; });

        tasks = lib.concatLists (
          lib.mapAttrsToList (_group: g: lib.mapAttrsToList (name: task: { inherit name task; }) g.tasks) groups
        );
        taskNames = map ({ name, ... }: name) tasks;
        taskRuntimePackages = lib.unique (lib.concatMap ({ task, ... }: task.runtimePackages) tasks);

        # Menu tasks are devshell commands too. A task whose `run` starts with
        # the task's own name asserts "this command already exists on PATH"
        # (motd, docs, previews…); every other task gets a generated wrapper
        # named after it that delegates to the menu fast path (`menu <name> …`,
        # same argument handling), bundled into packages.menu — so the names
        # the menu displays are directly invocable in any shell that includes
        # it. Delegating instead of inlining `run` keeps one execution contract.
        taskWrappers =
          let
            needsWrapper = { name, task }: task.run != null && builtins.head (lib.splitString " " task.run) != name;
            wrapped = lib.filter needsWrapper tasks;
          in
          assert lib.assertMsg (
            !lib.any ({ name, ... }: name == "menu") wrapped
          ) "prelude: a task named \"menu\" whose `run` is not `menu …` would shadow the menu itself";
          map (
            { name, ... }:
            # writeTextFile rather than writeShellApplication: task names may
            # contain ":" (valid in bin/ entries, invalid in store names).
            pkgs.writeTextFile {
              name = "prelude-task-${lib.replaceStrings [ ":" ] [ "-" ] name}";
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
          paths = [ menuBin ] ++ taskWrappers ++ taskRuntimePackages;
          passthru = {
            inherit taskNames taskWrappers taskRuntimePackages;
          };
          meta = {
            inherit (menuBin.meta) description;
            mainProgram = "menu";
          };
        };

        docsPkg = mkDocs deps (generatorConfig cfg.docs);

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
        (lib.mkIf cfg.docs.enable {
          packages.docs = docsPkg;
          apps.docs = mkApp docsPkg;
        })
      ];
  };
}
