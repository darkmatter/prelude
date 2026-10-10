/**
 * @fileoverview Command models a runnable command similar to the "scripts" field in
 * package.json. In our case, it is each line item that render in the command menu.
 */
import type * as Args from "./Args.ts";
import { validate } from "./internal/args.ts";

const TypeId: unique symbol = Symbol.for("@drkmttr/prelude/Command");

/** What `run` receives besides its parsed arguments. */
export interface RunContext {
  /** The key the command is mounted under, e.g. `db:migrate`. */
  readonly key: string;
  /** The raw argument words the values were parsed from. */
  readonly argv: readonly string[];
}

/** Metadata every command carries, as `prelude.commands.<key>` does in Nix. */
interface Info<Specs extends readonly Args.Spec[]> {
  description?: string;
  /** Menu group. Default: the key's segment before its first `:` or `/`, else `develop`. */
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

export type Definition<Specs extends readonly Args.Spec[]> =
  | FunctionDefinition<Specs>
  | ShellDefinition<Specs>;

/** A command from Command.make, ready to mount in a menu under a key. */
export type Command<Specs extends readonly Args.Spec[] = readonly Args.Spec[]> =
  Definition<Specs> & {
    readonly args: Specs;
    readonly [TypeId]: true;
  };

/**
 * Any command, whatever its arguments: the value type of a commands record.
 * `run` takes `never` because every typed `run` is assignable to that.
 */
export type Leaf = Info<readonly Args.Spec[]> & {
  readonly args: readonly Args.Spec[];
  readonly [TypeId]: true;
  readonly children?: never;
} & (
    | { run(args: never, context: RunContext): unknown; exec?: never }
    | { exec: string; run?: never }
  );

/** A native submenu. The current picker supports one level of child commands. */
export interface ParentDefinition extends Omit<Info<readonly []>, "args"> {
  children: Readonly<Record<string, Leaf>>;
  args?: never;
  run?: never;
  exec?: never;
}

export type Parent = Omit<ParentDefinition, "args"> & {
  readonly args: readonly [];
  readonly [TypeId]: true;
};

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
export function make<const Specs extends readonly Args.Spec[] = readonly []>(
  definition: Definition<Specs> | ParentDefinition,
): Command<Specs> | Parent {
  const modes = [typeof definition.run === "function", typeof definition.exec === "string", definition.children !== undefined];
  if (modes.filter(Boolean).length !== 1) {
    throw new Error(
      "prelude: Command.make needs exactly one of `run`, `exec`, or `children`",
    );
  }
  if (definition.children !== undefined) {
    if (definition.args !== undefined) throw new Error("prelude: submenu parents cannot declare arguments");
    const children = Object.entries(definition.children);
    if (children.length === 0) throw new Error("prelude: a submenu needs at least one child command");
    for (const [name, child] of children) {
      if (!is(child) || child.children !== undefined) {
        throw new Error(`prelude: submenu child "${name}" must be a leaf from Command.make()`);
      }
    }
    return Object.freeze({ ...definition, children: Object.freeze({ ...definition.children }), args: [] as const, [TypeId]: true as const });
  }
  const args = definition.args ?? ([] as unknown as Specs);
  validate(args, "prelude: Command.make");
  return Object.freeze({ ...definition, args, [TypeId]: true as const });
}

/** Whether a value was made by Command.make. */
export function is(value: unknown): value is Any {
  return (
    typeof value === "object" &&
    value !== null &&
    (value as { [TypeId]?: unknown })[TypeId] === true
  );
}
