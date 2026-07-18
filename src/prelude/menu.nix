# Command menu builder: a bubbletea TUI (internal/menu) fed a JSON config
# generated from the Nix options.
#
#   menu                 interactive picker: fuzzy filter, grouped results,
#                        tab-to-expand details, argument entry with chips
#   menu <name|key> …    fast path: run a command directly, extra args appended
#   menu list            print the grouped command table (non-interactive)
#   menu help            man-style manual generated from the config
#
# The Go binary is config-independent (one derivation shared by every menu
# configuration); each config becomes a JSON file baked into a thin wrapper.
{ lib
, writeShellApplication
, writeText
, buildGoModule
, symlinkJoin
, ...
}:

# Flat config: { theme?, palette?, colorProfile?, project?, commands?,
#                placeholder?, height?, execute?, width?, maxWidth? }
config:

let
  d = import ./defaults.nix;
  plib = import ./lib.nix { inherit lib; };

  pal = plib.resolvePalette (config.theme or d.theme) (config.palette or d.palette);
  colorProfile = config.colorProfile or d.colorProfile;
  project = config.project or d.project;
  groups = plib.normalizeCommandGroups (config.groupOrder or [ ]) (config.commands or d.commands);
  tasks = plib.flatCommands groups;

  m = d.menu // config;

  # --- validation ----------------------------------------------------------------

  safeName = n: builtins.match "[A-Za-z0-9:_.-]+" n != null;
  keys = lib.filter (k: k != null) (map (t: t.key) tasks);
  names = map (t: t.name) tasks;

  checkTasks =
    assert lib.assertMsg (tasks != [ ]) "menu: no commands configured — set `commands`";
    assert lib.assertMsg (lib.all safeName names)
      "menu: command names may only contain [A-Za-z0-9:_.-]";
    assert lib.assertMsg (lib.all safeName keys) "menu: command keys may only contain [A-Za-z0-9:_.-]";
    assert lib.assertMsg (lib.unique keys == keys) "menu: command keys must be unique";
    assert lib.assertMsg
      (
        lib.intersectLists keys names == [ ]
      ) "menu: command keys must not collide with command names";
    assert lib.assertMsg
      (
        !(lib.elem "list" (names ++ keys))
      ) "menu: \"list\" is reserved for `menu list`";
    # `help` is intentionally both a Prelude catalogue entry and the built-in
    # `menu help` fast path. Selecting it interactively executes that same path.
    true;

  # --- config payload ----------------------------------------------------------

  # The TUI is full-screen; width only informs the content cap.
  maxWidth =
    if m.maxWidth or null != null then
      m.maxWidth
    else if builtins.isInt (m.width or null) then
      m.width
    else
      0;

  orEmpty = v: if v == null then "" else v;

  jsonGroups = map
    (g: {
      title = g.title;
      tasks = map
        (t: {
          inherit (t)
            name
            label
            run
            description
            examples
            args
            ;
          key = orEmpty t.key;
          usage = orEmpty t.usage;
          details = orEmpty t.details;
        })
        g.tasks;
    })
    groups;

  configFile = writeText "prelude-menu.json" (
    builtins.toJSON {
      inherit project maxWidth colorProfile;
      placeholder = m.placeholder;
      height = m.height;
      execute = m.execute;
      palette = pal;
      groups = jsonGroups;
    }
  );

  # --- the TUI binary ------------------------------------------------------------

  menuTui = buildGoModule {
    pname = "prelude-menu";
    version = "0.1.0";
    src = ../.;
    subPackages = [ "cmd/menu" ];
    doCheck = false;
    vendorHash = "sha256-hKvYlJqQUQ3NrBRgWPZyvYhsCvceW1HbDRlzltKyCxQ=";
    ldflags = [
      "-s"
      "-w"
    ];
    meta = {
      description = "Interactive devshell command menu (bubbletea)";
      mainProgram = "menu";
    };
  };
  menuWrapper = writeShellApplication {
    name = "menu";
    text = ''
      exec ${lib.getExe menuTui} --config ${configFile} "$@"
    '';
  };

  xWrapper = writeShellApplication {
    name = "x";
    text = ''
      exec ${lib.getExe menuTui} --config ${configFile} --x "$@"
    '';
  };
in
assert checkTasks;
symlinkJoin {
  name = "menu";
  paths = [
    menuWrapper
    xWrapper
  ];
  meta = {
    description = "Interactive devshell command menu and x dispatcher (themed bubbletea TUI, configured by Nix)";
    mainProgram = "menu";
  };
}
