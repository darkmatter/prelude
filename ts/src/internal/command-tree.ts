import * as Args from "../Args.ts";
import type * as Command from "../Command.ts";

/** Resolve declared routes before FFI so function arguments retain their words. */
export function mount(commands: Readonly<Record<string, Command.Any>>) {
  const mounted = new Map(Object.entries(commands));
  const paths = new Map<string, readonly string[]>();
  for (const [parent, command] of Object.entries(commands)) {
    for (const [name, child] of Object.entries(command.children ?? {})) {
      const key = `${parent}/${name}`;
      if (mounted.has(key)) throw new Error(`prelude: duplicate mounted command key "${key}"`);
      mounted.set(key, child);
      paths.set(key, [parent, name]);
    }
  }
  const invocations = new Set<string>();
  for (const [key, command] of mounted) {
    if (command.exec !== undefined) {
      if (invocations.has(command.exec)) throw new Error(`prelude: duplicate canonical command invocation(s): ${command.exec}`);
      invocations.add(command.exec);
    }
    if (!paths.has(key) && command.shortcut !== undefined && paths.has(command.shortcut)) {
      throw new Error(`prelude: shortcut "${command.shortcut}" of ${key} collides with a child command key`);
    }
  }
  const keyFor = (word: string, scope: Readonly<Record<string, Command.Any>>) =>
    Object.hasOwn(scope, word) ? word : Object.keys(scope).find((key) => scope[key]!.shortcut === word);

  const resolve = (argv: readonly string[]) => {
    const [first, ...rest] = argv;
    let key = first === undefined ? undefined : keyFor(first, commands) ?? (paths.has(first) ? first : undefined);
    let words = rest;
    let command = key === undefined ? undefined : mounted.get(key);
    if (command?.children !== undefined && words.length > 0) {
      if (words[0] === "--") words = words.slice(1);
      if (words.length > 0) {
        const child = keyFor(words[0]!, command.children);
        if (child === undefined) throw new Args.ParseError(`unknown subcommand "${words[0]}" of ${key}`);
        key = `${key}/${child}`;
        words = words.slice(1);
        command = mounted.get(key);
      }
    }
    return { key, command, words, native: key === undefined ? argv : [...(paths.get(key) ?? [key]), ...words] };
  };

  return { mounted, resolve };
}
