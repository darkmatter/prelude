import * as Command from "../Command.ts";

// The catalogue rules of src/prelude/command-catalogue.nix and the checks in
// src/prelude/menu.nix, for commands keyed by their public name. The
// conformance test compares this port's output with Nix's for shared fixtures.

/**
 * One node of the command tree: a mounted command, or a parent only its
 * subcommands imply (`db` for `db migrate`), which has no key or command.
 */
export interface Node {
  /** The words after the dispatcher: `["db", "migrate"]`. */
  readonly path: readonly string[];
  /** The canonical name, the words joined by single spaces: `db migrate`. */
  readonly name: string;
  /** The last word, shown in the menu. */
  readonly label: string;
  /** Menu group: "" lists without a heading. Only top-level nodes are grouped. */
  readonly group: string;
  /** The key the command is mounted under: `db/migrate` or `db migrate`. */
  readonly key?: string;
  readonly command?: Command.Any;
  /** Subcommands, sorted by label, then name. */
  readonly children: readonly Node[];
}

/** A node some key mounts a command at. */
export type Declared = Node & { readonly key: string; readonly command: Command.Any };

export interface Group {
  readonly title: string;
  /** Top-level nodes; subcommands sit under them. */
  readonly nodes: readonly Node[];
}

const keyPattern = /^[A-Za-z0-9:_.-]+([ /][A-Za-z0-9:_.-]+)*$/;
// Single-key accelerators are one word, checked as menu.nix checks them.
const safeShortcut = /^[A-Za-z0-9:/_.-]+$/;

/** Byte order, as Nix compares strings. */
export function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/**
 * Mirrors `commandIdentity`. A key is words separated by one space or `/`:
 * `db migrate` and `db/migrate` both name `migrate` under `db`, and `:` is an
 * ordinary name character. Keys are never parsed for a group: a command's
 * group is the explicit one, else "" (listed above every heading), except
 * that Prelude's own `x` and `docs` default to `prelude`. Only top-level
 * commands take a group; a subcommand lists under its parent.
 */
export function identity(
  key: string,
  explicitGroup?: string,
): { path: string[]; name: string; label: string; group: string } {
  if (!keyPattern.test(key)) {
    throw new Error(
      `prelude: command key "${key}" must be words of letters, digits, : _ . - separated by a single space or /`,
    );
  }
  const path = key.split(/[ /]/);
  const name = path.join(" ");
  const root = path[0]!;
  const subcommand = path.length > 1;
  if (subcommand && explicitGroup !== undefined) {
    throw new Error(`prelude: "${name}" is a subcommand of "${root}"; set group on "${root}" instead`);
  }
  const group = explicitGroup ?? (!subcommand && (name === "x" || name === "docs") ? "prelude" : "");
  return { path, name, label: path.at(-1)!, group };
}

/**
 * Whether choosing a node only opens its subcommands: it has some and nothing
 * of its own to run. One with `run` or `exec` is a runnable parent instead.
 */
export function isContainer(node: Node): boolean {
  return node.children.length > 0 && node.command?.run === undefined && node.command?.exec === undefined;
}

/** A node's description; a container without one counts its subcommands. */
export function describe(node: Node): string {
  const own = node.command?.description ?? "";
  if (own !== "" || !isContainer(node)) return own;
  const count = node.children.length;
  return count === 1 ? "1 subcommand" : `${count} subcommands`;
}

/**
 * How people run a node from a shell: Nix's `command`. Nix runs a node bare
 * when its root is a declared top-level command without `:`, which gets a
 * PATH wrapper, and as `x <words>` otherwise. A TypeScript app has no PATH
 * wrappers, only its dispatcher, so every node takes the dispatch form.
 */
export function invocation(node: Node, dispatcher: string): string {
  return `${dispatcher} ${node.name}`;
}

/** Whether a key mounts a command at a node, rather than its subcommands implying it. */
export function isDeclared(node: Node): node is Declared {
  return node.command !== undefined;
}

/** Every node, parents before their subcommands. */
export function flatten(nodes: readonly Node[]): Node[] {
  return nodes.flatMap((node) => [node, ...flatten(node.children)]);
}

/**
 * Builds the command tree and groups its top-level nodes the way
 * `normalizeCommandGroups` does: commands without a group first (no heading,
 * so they can't read as part of the group above), then the `prelude` group,
 * then groupOrder, then the rest alphabetically; each group sorted by label,
 * then name.
 */
export function catalogue(commands: Readonly<Record<string, Command.Any>>, groupOrder: readonly string[] = []): Group[] {
  if (new Set(groupOrder).size !== groupOrder.length) {
    throw new Error("prelude: groupOrder must not contain duplicates");
  }
  const entries = mount(commands);
  const nodes = level(0, entries);
  validate(entries, flatten(nodes));

  const available = [...new Set(nodes.map((node) => node.group))];
  const ungrouped = available.includes("") ? [""] : [];
  const named = available.filter((group) => group !== "");
  const preferred = [...new Set(["prelude", ...groupOrder])].filter((group) => named.includes(group));
  const remaining = named.filter((group) => !preferred.includes(group)).sort(compare);
  return [...ungrouped, ...preferred, ...remaining].map((title) => ({
    title,
    nodes: nodes.filter((node) => node.group === title),
  }));
}

const childName = /^[A-Za-z0-9:_.-]+$/;

/**
 * Every command and the key it is mounted under. A parent's `children` are
 * shorthand for keys below it: each mounts at `<key>/<name>` and lists under
 * the parent, so a child's own `group` does not apply.
 */
function mount(commands: Readonly<Record<string, Command.Any>>): Declared[] {
  const entries: Declared[] = [];
  const children: Declared[] = [];
  for (const [key, command] of Object.entries(commands)) {
    if (!Command.is(command)) {
      throw new Error(`prelude: commands["${key}"] is not a command; create it with Command.make()`);
    }
    entries.push({ ...identity(key, command.group), key, command, children: [] });
    const declaredChildren: Readonly<Record<string, Command.Leaf>> = command.children ?? {};
    for (const [name, child] of Object.entries(declaredChildren)) {
      if (!childName.test(name)) {
        throw new Error(
          `prelude: submenu child "${name}" of ${key} must be one command key word of letters, digits, : _ . -`,
        );
      }
      const mounted = `${key}/${name}`;
      children.push({ ...identity(mounted), key: mounted, command: child, children: [] });
    }
  }
  const declared = new Set(entries.map((entry) => entry.name));
  for (const child of children) {
    if (declared.has(child.name)) throw new Error(`prelude: duplicate mounted command key "${child.key}"`);
  }
  return [...entries, ...children];
}

/**
 * Mirrors `buildNodes`: the nodes one level below a shared path, at word
 * `depth`. Each is the command declared for its path, or else a parent
 * synthesized for a path only subcommands declare, with its own subcommands
 * below it.
 */
function level(depth: number, entries: readonly Declared[]): Node[] {
  const words = [...new Set(entries.map((entry) => entry.path[depth]!))];
  return words
    .map((word): Node => {
      const members = entries.filter((entry) => entry.path[depth] === word);
      const declared = members.find((entry) => entry.path.length === depth + 1);
      const children = level(
        depth + 1,
        members.filter((entry) => entry.path.length > depth + 1),
      );
      return declared === undefined
        ? { ...identity(members[0]!.path.slice(0, depth + 1).join(" ")), children }
        : { ...declared, children };
    })
    .sort((a, b) => compare(a.label, b.label) || compare(a.name, b.name));
}

function validate(entries: readonly Declared[], nodes: readonly Node[]): void {
  // `db/migrate` and `db migrate` are one command; declaring both is a typo.
  const sameName = [...Map.groupBy(entries, (entry) => entry.name)]
    .filter(([, group]) => group.length > 1)
    .sort(([a], [b]) => compare(a, b))
    .map(([, group]) => group.map((entry) => `"${entry.key}"`).sort(compare).join(", "));
  if (sameName.length > 0) {
    throw new Error(`prelude: these keys name the same command: ${sameName.join("; ")}`);
  }

  // A Nix command without `exec` runs its last word from PATH; a TypeScript
  // one without `run` or `exec` has nothing to run unless it holds subcommands.
  for (const { key, command, children } of nodes) {
    if (command !== undefined && command.run === undefined && command.exec === undefined && children.length === 0) {
      throw new Error(
        `prelude: commands["${key}"] has neither \`run\` nor \`exec\`; only a command with subcommands may leave both out`,
      );
    }
  }

  const names = new Set(nodes.map((node) => node.name));
  const shortcuts = new Map<string, string>();
  for (const { key, command } of entries) {
    const shortcut = command.shortcut;
    if (shortcut === undefined) continue;
    if (!safeShortcut.test(shortcut)) {
      throw new Error(`prelude: shortcut "${shortcut}" of ${key} may only contain letters, digits, and : / _ . -`);
    }
    const owner = shortcuts.get(shortcut);
    if (owner !== undefined) throw new Error(`prelude: shortcut "${shortcut}" is used by both ${owner} and ${key}`);
    if (names.has(shortcut)) throw new Error(`prelude: shortcut "${shortcut}" of ${key} collides with a command name`);
    shortcuts.set(shortcut, key);
  }

  // Shell commands keep one canonical invocation each, as in Nix.
  const seen = new Set<string>();
  const duplicates = new Set<string>();
  for (const { command } of entries) {
    if (command.exec === undefined) continue;
    if (seen.has(command.exec)) duplicates.add(command.exec);
    seen.add(command.exec);
  }
  if (duplicates.size > 0) {
    throw new Error(`prelude: duplicate canonical command invocation(s): ${[...duplicates].join(", ")}`);
  }
}

/**
 * Mirrors `selectCommands`: commands at any depth with a `motd` position,
 * ordered by it, then name.
 */
export function gettingStarted(groups: readonly Group[]): Declared[] {
  return flatten(groups.flatMap((group) => group.nodes))
    .filter(isDeclared)
    .filter((node) => node.command.motd !== undefined)
    .sort((a, b) => a.command.motd! - b.command.motd! || compare(a.name, b.name));
}
