import * as Command from "../Command.ts";

// The catalogue rules of src/prelude/command-catalogue.nix and the checks in
// src/prelude/menu.nix, for commands keyed by their public name. The
// conformance test compares this port's output with Nix's for shared fixtures.

/** A mounted command with its presentation identity. */
export interface Entry {
  readonly key: string;
  readonly group: string;
  readonly label: string;
  readonly command: Command.Any;
}

export interface Group {
  readonly title: string;
  readonly entries: readonly Entry[];
}

const safeName = /^[A-Za-z0-9:/_.-]+$/;

/** Byte order, as Nix compares strings. */
export function compare(a: string, b: string): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

/**
 * Mirrors `commandIdentity`: the first `:` or `/` splits the menu group from
 * the displayed label while the key stays whole (`go:test` and `go/test` →
 * group `go`, label `test`); later separators stay in the label. A key with
 * neither has no group (""), so the menu lists it without a heading. An
 * explicit group overrides the inferred one, `""` included.
 */
export function identity(key: string, explicitGroup?: string): { group: string; label: string } {
  if (!safeName.test(key)) {
    throw new Error(`prelude: command key "${key}" may only contain letters, digits, and : / _ . -`);
  }
  const separator = key.search(/[:/]/);
  const grouped = separator !== -1;
  const group = explicitGroup ?? (grouped ? key.slice(0, separator) : "");
  const label = grouped ? key.slice(separator + 1) : key;
  // A separator needs a name on each side, unless an explicit group replaces
  // the one before it.
  if (label === "" || (grouped && explicitGroup === undefined && group === "")) {
    throw new Error(`prelude: command key "${key}" must have non-empty segments around its first : or /`);
  }
  return { group, label };
}

/**
 * Groups commands the way `normalizeCommandGroups` does: commands without a
 * group first (no heading, so they can't read as part of the group above),
 * then the `prelude` group, then groupOrder, then the rest alphabetically;
 * each group sorted by label.
 */
export function catalogue(commands: Readonly<Record<string, Command.Any>>, groupOrder: readonly string[] = []): Group[] {
  if (new Set(groupOrder).size !== groupOrder.length) {
    throw new Error("prelude: groupOrder must not contain duplicates");
  }
  const entries: Entry[] = Object.entries(commands).map(([key, command]) => {
    if (!Command.is(command)) {
      throw new Error(`prelude: commands["${key}"] is not a command; create it with Command.make()`);
    }
    return { key, command, ...identity(key, command.group) };
  });
  validate(entries);

  const available = [...new Set(entries.map((entry) => entry.group))];
  const ungrouped = available.includes("") ? [""] : [];
  const named = available.filter((group) => group !== "");
  const preferred = [...new Set(["prelude", ...groupOrder])].filter((group) => named.includes(group));
  const remaining = named.filter((group) => !preferred.includes(group)).sort(compare);
  return [...ungrouped, ...preferred, ...remaining].map((title) => ({
    title,
    entries: entries
      .filter((entry) => entry.group === title)
      .sort((a, b) => compare(a.label, b.label) || compare(a.key, b.key)),
  }));
}

function validate(entries: readonly Entry[]): void {
  const keys = new Set(entries.map((entry) => entry.key));
  const shortcuts = new Map<string, string>();
  for (const { key, command } of entries) {
    const shortcut = command.shortcut;
    if (shortcut === undefined) continue;
    if (!safeName.test(shortcut)) {
      throw new Error(`prelude: shortcut "${shortcut}" of ${key} may only contain letters, digits, and : / _ . -`);
    }
    const owner = shortcuts.get(shortcut);
    if (owner !== undefined) throw new Error(`prelude: shortcut "${shortcut}" is used by both ${owner} and ${key}`);
    if (keys.has(shortcut)) throw new Error(`prelude: shortcut "${shortcut}" of ${key} collides with a command key`);
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

/** Mirrors `selectCommands`: commands with a `motd` position, ordered by it, then key. */
export function gettingStarted(groups: readonly Group[]): Entry[] {
  return groups
    .flatMap((group) => group.entries)
    .filter((entry) => entry.command.motd !== undefined)
    .sort((a, b) => a.command.motd! - b.command.motd! || compare(a.key, b.key));
}
