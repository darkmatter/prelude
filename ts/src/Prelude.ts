import * as Command from "./Command.ts";
import * as Docs from "./Docs.ts";
import * as Menu from "./Menu.ts";
import * as Motd from "./Motd.ts";
import type * as Palette from "./Palette.ts";
import { defaultDispatcher } from "./internal/process.ts";

/** One app: its commands, and optionally a MOTD and docs, sharing a look. */
export interface Options extends Palette.Options {
  project?: string;
  /** How people run this app from a shell (`acme`). Default: derived from process.argv. */
  dispatcher?: string;
  commands: Readonly<Record<string, Command.Any>>;
  groupOrder?: readonly string[];
  menu?: Pick<Menu.Options, "placeholder" | "height" | "maxWidth" | "just" | "scripts">;
  motd?: Omit<Motd.Options, keyof Palette.Options | "project" | "commands" | "groupOrder" | "dispatcher">;
  docs?: Omit<Docs.Options, keyof Palette.Options | "project">;
}

export interface Prelude {
  readonly menu: Menu.Menu;
  /** The welcome banner, when `motd` is configured. */
  readonly motd: Motd.Motd | undefined;
  /** The docs viewer, when `docs` is configured. */
  readonly docs: Docs.Docs | undefined;
  /**
   * The app's command line: no words opens the picker, `<key> [args…]` runs a
   * command (including the built-in `motd` and `docs`), `--list` prints them.
   */
  main(argv?: readonly string[]): Promise<number>;
}

export function make(options: Options): Prelude {
  const dispatcher = options.dispatcher ?? defaultDispatcher();
  const { project, theme, palette, colorProfile, groupOrder } = options;
  const look = { project, theme, palette, colorProfile };

  // Prelude's own entry points join the catalogue, as the devshell lists
  // `motd` and `docs`; a command of yours under the same key wins.
  let motd: Motd.Motd | undefined;
  let docs: Docs.Docs | undefined;
  const builtins: Record<string, Command.Any> = {};
  if (options.motd) {
    builtins.motd = Command.make({ group: "prelude", description: "reprint the welcome banner", run: () => motd!.print() });
  }
  if (options.docs) {
    builtins.docs = Command.make({ group: "prelude", description: "browse the documentation", run: () => docs!.open() });
  }
  const commands = { ...builtins, ...options.commands };

  motd =
    options.motd &&
    Motd.make({
      ...look,
      ...options.motd,
      commands,
      groupOrder,
      dispatcher,
      // Like the devshell's shortcut chips: the ways back to the menu and docs.
      shortcuts: options.motd.shortcuts ?? [{ command: dispatcher }, ...(options.docs ? [{ command: `${dispatcher} docs` }] : [])],
    });
  docs = options.docs && Docs.make({ ...look, ...options.docs });
  const menu = Menu.make({ ...look, ...options.menu, dispatcher, commands, groupOrder });

  return { menu, motd, docs, main: (argv) => menu.dispatch(argv) };
}
