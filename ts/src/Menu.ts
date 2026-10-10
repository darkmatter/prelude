import { basename } from "node:path";

import * as Args from "./Args.ts";
import type * as Command from "./Command.ts";
import * as Palette from "./Palette.ts";
import { catalogue, children, gettingStarted, type Entry } from "./internal/catalogue.ts";
import { mount } from "./internal/command-tree.ts";
import { call } from "./internal/ffi.ts";
import { defaults } from "./internal/generated.ts";
import { resolveColorProfile } from "./internal/palette.ts";
import { announce, complain, defaultDispatcher, exitStatus, runShell } from "./internal/process.ts";

export interface Options extends Palette.Options {
  /** Shown in the title bar and prompt. Default: the working directory's name. */
  project?: string;
  /** How people run this app from a shell (`acme`). Default: derived from process.argv. */
  dispatcher?: string;
  /** Commands keyed by their public name; the first `:` or `/` groups them (`db:migrate`, `db/migrate`). */
  commands: Readonly<Record<string, Command.Any>>;
  /** Preferred group order; unlisted groups follow alphabetically. */
  groupOrder?: readonly string[];
  /** Filter input placeholder. */
  placeholder?: string;
  /** Filter list height in rows. */
  height?: number;
  /** Maximum menu width; null for unbounded. */
  maxWidth?: number | null;
  /** Import public Justfile recipes when the picker opens, like `prelude.menu.just`. */
  just?: { enable?: boolean; justfile?: string | null; group?: string };
  /**
   * Import package.json scripts when the picker opens, like
   * `prelude.menu.scripts`. Each runs exactly as written, from its
   * package.json directory with node_modules/.bin ahead of PATH. Default: the
   * nearest package.json at or above the working directory.
   */
  scripts?: { enable?: boolean; packageJson?: string | null; group?: string };
}

type TaskArg = { token: string; description: string; required: boolean; boolean: boolean; options: string[]; default: string | null };

export interface Task {
  name: string;
  label: string;
  run: string;
  command: string;
  description: string;
  key: string;
  usage: string;
  details: string;
  examples: string[];
  args: TaskArg[];
  children?: Task[];
}

/** The menu's JSON boundary, internal/menu.Config in Go. */
export interface Config {
  project: string;
  placeholder: string;
  height: number;
  maxWidth: number;
  execute: boolean;
  colorProfile: Palette.ColorProfile;
  palette: Palette.Palette;
  groups: { title: string; tasks: Task[] }[];
  motdCommands: { name: string; command: string; description: string }[];
  just: { enable: boolean; justfile: string | null; group: string };
  scripts: { enable: boolean; packageJson: string | null; group: string };
  dispatcher: string;
}

/** Where a command came from: this app's own commands, or an import. */
export type Source = "declared" | "just" | "scripts";

/** What the picker resolved. */
export interface Selection {
  /** The chosen command's key. */
  readonly key: string;
  /** Argument text entered for it; "" when none. */
  readonly line: string;
  /** The shell form: exec text plus the line. */
  readonly shell: string;
  /** Where the command came from. Only a declared one can be a function of this app. */
  readonly source: Source;
  /** The directory the shell form runs in; absent means this process's. */
  readonly dir?: string;
  /** Directories put ahead of PATH for the shell form, nearest first. */
  readonly pathPrefix?: readonly string[];
}

export interface Menu {
  /** The config handed to the Go menu. */
  readonly config: Config;
  /**
   * Opens the picker and returns what the person chose without running it,
   * or null when they leave. With args, resolves them like `x` does and
   * opens only what still needs input.
   */
  select(args?: readonly string[]): Selection | null;
  /** The `--list` table as text. */
  list(width?: number): string;
  /** Runs a selection: a function command in this process, shell text in bash. */
  run(selection: Selection): Promise<number>;
  /** Opens the picker and runs the choice. Resolves with, and sets process.exitCode to, its exit status. */
  launch(): Promise<number>;
  /**
   * The app's command line, with `x` semantics: no words opens the picker,
   * `<key> [args…]` runs a command, `--list` prints the table. Resolves with,
   * and sets process.exitCode to, the exit status.
   */
  dispatch(argv?: readonly string[]): Promise<number>;
}

/** Builds the menu config the way src/prelude/menu.nix does, from TypeScript commands. */
function buildConfig(options: Options, dispatcher: string): Config {
  const groups = catalogue(options.commands, options.groupOrder);
  const just = {
    enable: options.just?.enable ?? defaults.menu.just.enable,
    justfile: options.just?.justfile ?? defaults.menu.just.justfile,
    group: options.just?.group ?? defaults.menu.just.group,
  };
  const scripts = {
    enable: options.scripts?.enable ?? defaults.menu.scripts.enable,
    packageJson: options.scripts?.packageJson ?? defaults.menu.scripts.packageJson,
    group: options.scripts?.group ?? defaults.menu.scripts.group,
  };
  if (groups.length === 0 && !just.enable && !scripts.enable) {
    throw new Error("prelude: no commands configured; add commands or enable just or scripts");
  }
  const height = options.height ?? defaults.menu.height;
  if (!Number.isInteger(height) || height <= 0) {
    throw new Error(`prelude: menu height must be a positive integer, got ${height}`);
  }
  return {
    project: options.project ?? basename(process.cwd()),
    placeholder: options.placeholder ?? defaults.menu.placeholder,
    height,
    maxWidth: options.maxWidth === undefined ? defaults.menu.maxWidth : (options.maxWidth ?? 0),
    // The Go menu only consults this before exec'ing; as a library it never
    // execs, because the host runs every selection itself.
    execute: true,
    colorProfile: resolveColorProfile(options.colorProfile),
    palette: Palette.resolve(options.theme, options.palette),
    groups: groups.map((group) => ({ title: group.title, tasks: group.entries.map((entry) => task(entry, dispatcher)) })),
    motdCommands: gettingStarted(groups).map((entry) => ({
      name: entry.key,
      command: `${dispatcher} ${entry.key}`,
      description: entry.command.description ?? "",
    })),
    just,
    scripts,
    dispatcher,
  };
}

function task(entry: Entry, dispatcher: string): Task {
  const { key, label, command } = entry;
  const invocation = `${dispatcher} ${key}`;
  return {
    name: key,
    label,
    // A function command has no shell text; its preview shows how to run it
    // from a shell instead.
    run: command.exec ?? invocation,
    command: invocation,
    description: command.description ?? "",
    key: command.shortcut ?? "",
    usage: command.usage ?? "",
    details: command.details ?? "",
    examples: [...(command.examples ?? [])],
    args: command.args.map(taskArg),
    ...(command.children === undefined ? {} : { children: children(entry).map((child) => task(child, dispatcher)) }),
  };
}

function taskArg(spec: Args.Spec): TaskArg {
  return {
    token: spec.token,
    description: spec.description ?? "",
    required: spec.required ?? false,
    boolean: spec.boolean ?? false,
    options: [...(spec.options ?? [])],
    default: spec.default === undefined ? null : String(spec.default),
  };
}

/** A command menu over TypeScript commands, drawn by prelude's Go menu. */
export function make(options: Options): Menu {
  const dispatcher = options.dispatcher ?? defaultDispatcher();
  const config = buildConfig(options, dispatcher);
  const { mounted, resolve } = mount(options.commands);
  const { palette } = config;

  const select = (args: readonly string[] = []): Selection | null => {
    const chosen = call<{
      name: string;
      line: string;
      command: string;
      source: Source;
      dir?: string;
      pathPrefix?: string[];
    } | null>("prelude_menu_select", { config, args: resolve(args).native });
    return (
      chosen && {
        key: chosen.name,
        line: chosen.line,
        shell: chosen.command,
        source: chosen.source,
        dir: chosen.dir,
        pathPrefix: chosen.pathPrefix,
      }
    );
  };

  const list = (width = process.stdout.columns ?? 80): string => call<string>("prelude_menu_list", { config, width });

  const invoke = (key: string, command: Command.Any, argv: readonly string[]) => {
    // Parse before anything runs so a typo never half-starts a command. The
    // values match the command's own declarations, which Command.Any erases.
    const args = Args.parse(command.args, argv) as never;
    return async () => exitStatus(await command.run!(args, { key, argv }));
  };

  // A picker choice, or words Go resolved like `x` (shortcuts of shell
  // commands, Justfile recipes and module routes, typed extra arguments).
  // Only a declared selection can name one of this app's functions; an
  // import that shares a key still runs its own shell text.
  const prepareSelection = (selection: Selection): (() => Promise<number>) => {
    const command = selection.source === "declared" ? mounted.get(selection.key) : undefined;
    if (command?.children !== undefined) throw new Args.ParseError(`select a subcommand of ${selection.key}`);
    if (command?.run === undefined) {
      return () => {
        announce(palette, selection.shell);
        return runShell(selection.shell, selection);
      };
    }
    const line = selection.line === "" ? "" : ` ${selection.line}`;
    const run = invoke(selection.key, command, Args.split(selection.line));
    return () => {
      announce(palette, `${dispatcher} ${selection.key}${line}`);
      return run();
    };
  };

  const prepare = (argv: readonly string[]): (() => Promise<number>) => {
    const { key, command, words } = resolve(argv);
    // Function commands parse their own words, so quoting survives intact and
    // `--` ends option parsing; with none given, argument entry in the picker
    // collects them.
    if (key !== undefined && command?.run !== undefined && (words.length > 0 || command.args.length === 0)) {
      return invoke(key, command, words);
    }
    const selection = select(argv);
    return selection === null ? async () => 0 : prepareSelection(selection);
  };

  const finish = (status: number): number => {
    process.exitCode = status;
    return status;
  };

  const dispatch = async (argv: readonly string[] = process.argv.slice(2)): Promise<number> => {
    const [first] = argv;
    if (first === "--list" || first === "-l") {
      process.stdout.write(list());
      return finish(0);
    }
    if (first === "--help" || first === "-h") {
      process.stdout.write(`usage: ${dispatcher} [--list | <command> [args…]]\n\n${list()}`);
      return finish(0);
    }
    let run: () => Promise<number>;
    try {
      run = prepare(argv);
    } catch (error) {
      complain(palette, `${dispatcher}: ${error instanceof Error ? error.message : String(error)}`);
      return finish(error instanceof Args.ParseError ? 2 : 1);
    }
    return finish(await run());
  };

  return {
    config,
    select,
    list,
    run: (selection) => prepareSelection(selection)(),
    launch: () => dispatch([]),
    dispatch,
  };
}
