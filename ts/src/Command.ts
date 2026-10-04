/**
 * @fileoverview Command models a runnable command similar to the "scripts" field in
 * package.json. In our case, it is each line item that render in the command menu.
 */
import type * as Args from "./Args.ts";
import { validate } from "./internal/args.ts";

const TypeId: unique symbol = Symbol.for("@drkmttr/prelude/Command");

/** What `run` receives besides its parsed arguments. */
export interface RunContext {
  /** The key the command is mounted under, e.g. `db/migrate`. */
  readonly key: string;
  /** The raw argument words the values were parsed from. */
  readonly argv: readonly string[];
}

/** Metadata every command carries, as `prelude.commands.<key>` does in Nix. */
interface Info<Specs extends readonly Args.Spec[]> {
  description?: string;
  /**
   * Menu group: the heading the command lists under. Keys are never parsed
   * for one; without it the command lists above every heading. Only a
   * top-level command takes a group: a subcommand (`db migrate`) lists under
   * its parent, so set the group there.
   */
  group?: string;
  /** Single-character accelerator in the picker (Nix: `key`). */
  shortcut?: string;
  usage?: string;
  details?: string;
  examples?: readonly string[];
  /** Position on the MOTD's Getting Started list; omit to leave it off. */
  motd?: number;
  /** Declared arguments, parsed and handed to `run` as typed values. */
  args?: Specs;
}

/** A command implemented by a TypeScript function. */
export interface FunctionDefinition<Specs extends readonly Args.Spec[]> extends Info<Specs> {
  /** Runs in this process. A returned number becomes the exit status. */
  run(args: Args.Values<Specs>, context: RunContext): unknown;
  exec?: never;
  children?: never;
}

/** A command implemented by shell text, like `exec` in Nix. */
export interface ShellDefinition<Specs extends readonly Args.Spec[]> extends Info<Specs> {
  /** Run with bash; argument text entered for it is appended as typed. */
  exec: string;
  run?: never;
  children?: never;
}

/**
 * A command that only holds subcommands, like a Nix command without `exec`:
 * choosing it opens them. Its subcommands are other keys below it (`db`
 * beside `db migrate`), or `children`, shorthand that mounts each one at
 * `<key>/<name>`. Declare a parent to give its subcommands a group or a
 * description; a parent only subcommand keys imply has neither.
 */
export interface ParentDefinition extends Omit<Info<readonly []>, "args"> {
  /** Subcommands mounted below this command's key, one word each. */
  children?: Readonly<Record<string, Leaf>>;
  args?: never;
  run?: never;
  exec?: never;
}

export type Definition<Specs extends readonly Args.Spec[]> = FunctionDefinition<Specs> | ShellDefinition<Specs>;

/** A command from Command.make, ready to mount in a menu under a key. */
export type Command<Specs extends readonly Args.Spec[] = readonly Args.Spec[]> =
  Definition<Specs> & {
    readonly args: Specs;
    readonly [TypeId]: true;
  };

/**
 * A command that runs, whatever its arguments. `run` takes `never` because
 * every typed `run` is assignable to that.
 */
export type Leaf = Info<readonly Args.Spec[]> & {
  readonly args: readonly Args.Spec[];
  readonly [TypeId]: true;
  readonly children?: never;
} & (
    | { run(args: never, context: RunContext): unknown; exec?: never }
    | { exec: string; run?: never }
  );

/** A parent from Command.make: it opens its subcommands. */
export type Parent = Omit<ParentDefinition, "args"> & {
  readonly args: readonly [];
  readonly [TypeId]: true;
};

/** Any command: the value type of a commands record. */
export type Any = Leaf | Parent;

/**
 * Declares a command next to the code it wraps. Its key comes from where it
 * is mounted, so a module can export it without claiming a public name:
 *
 *     export const preludeCommand = Command.make({
 *       description: "run the dev server",
 *       args: [{ token: "--port", type: "number", default: 3000 }],
 *       run: (args) => serve(args.port),
 *     })
 */
export function make(definition: ParentDefinition): Parent;
export function make<const Specs extends readonly Args.Spec[] = readonly []>(
  definition: Definition<Specs>,
): Command<Specs>;
export function make(definition: Definition<readonly Args.Spec[]> | ParentDefinition): Any {
  const { run, exec, children } = definition as { run?: unknown; exec?: unknown; children?: unknown };
  if (
    (run !== undefined && typeof run !== "function") ||
    (exec !== undefined && typeof exec !== "string") ||
    (run !== undefined && exec !== undefined)
  ) {
    throw new Error("prelude: Command.make takes at most one of `run` (a function) or `exec` (shell text)");
  }
  if (children !== undefined && (run !== undefined || exec !== undefined)) {
    throw new Error("prelude: Command.make takes exactly one of `run`, `exec` or `children`");
  }
  if (definition.children !== undefined) {
    if (definition.args !== undefined) throw new Error("prelude: submenu parents cannot declare arguments");
    const entries = Object.entries(definition.children);
    if (entries.length === 0) throw new Error("prelude: a submenu needs at least one child command");
    for (const [name, child] of entries) {
      if (!is(child) || child.children !== undefined) {
        throw new Error(`prelude: submenu child "${name}" must be a leaf from Command.make()`);
      }
    }
    return Object.freeze({
      ...definition,
      children: Object.freeze({ ...definition.children }),
      args: [] as const,
      [TypeId]: true as const,
    });
  }
  const args = definition.args ?? [];
  validate(args, "prelude: Command.make");
  return Object.freeze({ ...definition, args, [TypeId]: true as const }) as Any;
}

/** Whether a value was made by Command.make. */
export function is(value: unknown): value is Any {
  return (
    typeof value === "object" &&
    value !== null &&
    (value as { [TypeId]?: unknown })[TypeId] === true
  );
}
