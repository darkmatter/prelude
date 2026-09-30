import { expect, test } from "bun:test";

import { Command, Menu, Motd } from "@drkmttr/prelude";

import expectedJson from "./fixtures/conformance.expected.json";
import fixturesJson from "./fixtures/conformance.json";

// The TypeScript port must produce the JSON Nix produces for the same inputs.
// conformance.expected.json is Nix's own output for conformance.json
// (regenerate with `x ts:sync`; the ts-generated-fresh check keeps it honest).

type NixCommand = Record<string, unknown> & { key?: string };
type Fixture = Record<string, unknown> & { commands?: Record<string, NixCommand> };
const fixtures = fixturesJson as unknown as { menu: Record<string, Fixture>; motd: Record<string, Fixture> };
const expected = expectedJson as unknown as { menu: Record<string, unknown>; motd: Record<string, unknown> };
const dispatcher = "fixture";

/** Nix-shaped commands as TypeScript ones: Nix's `key` accelerator is `shortcut` here. */
function commands(nix: Record<string, NixCommand> = {}): Record<string, Command.Any> {
  return Object.fromEntries(
    Object.entries(nix).map(([key, { key: shortcut, ...fields }]) => [
      key,
      Command.make({ ...fields, ...(shortcut === undefined ? {} : { shortcut }) } as Parameters<typeof Command.make>[0]),
    ]),
  );
}

type Json = { [key: string]: Json } | Json[] | string | number | boolean | null;

/**
 * Drops the differences that are intentional: a TypeScript app is run as
 * `<dispatcher> <key>` rather than through the devshell's `x` or PATH, and
 * `async` means nothing on a static badge (Nix defaults it to true; the port
 * never sets it, because its checks run inline).
 */
function withoutIntendedDifferences(config: unknown): Json {
  const copy = structuredClone(config) as { [key: string]: Json };
  delete copy.dispatcher;
  const dropCommand = (rows: Json | undefined) => {
    for (const row of (rows ?? []) as { command?: Json }[]) delete row.command;
  };
  for (const group of (copy.groups ?? []) as { tasks: Json }[]) dropCommand(group.tasks);
  dropCommand(copy.motdCommands);
  dropCommand(copy.commands);
  const header = copy.header as { status?: { check: string; async?: Json }[] } | undefined;
  for (const item of header?.status ?? []) {
    if (item.check === "") delete item.async;
  }
  return copy;
}

for (const [name, { commands: nix, ...options }] of Object.entries(fixtures.menu)) {
  test(`menu config "${name}" matches Nix`, () => {
    const { config } = Menu.make({ ...(options as Omit<Menu.Options, "commands">), commands: commands(nix), dispatcher });
    expect(withoutIntendedDifferences(config)).toEqual(withoutIntendedDifferences(expected.menu[name]));
  });
}

for (const [name, { commands: nix, ...options }] of Object.entries(fixtures.motd)) {
  test(`motd config "${name}" matches Nix`, () => {
    const status = (options.header as { status?: Record<string, Record<string, unknown>> } | undefined)?.status ?? {};
    const header = {
      ...(options.header as object),
      // `async` has no TypeScript counterpart: checks always run inline.
      status: Object.fromEntries(Object.entries(status).map(([key, { async: _, ...item }]) => [key, item])),
    };
    const { config } = Motd.make({ ...(options as Motd.Options), header, commands: commands(nix), dispatcher });
    expect(withoutIntendedDifferences(config)).toEqual(withoutIntendedDifferences(expected.motd[name]));
  });
}

test("the TypeScript invocation form is the dispatcher plus the key", () => {
  const { config } = Menu.make({ commands: commands(fixtures.menu.catalogue!.commands), dispatcher: "acme" });
  const tasks = config.groups.flatMap((group) => group.tasks);
  expect(tasks.find((task) => task.name === "db:migrate")?.command).toBe("acme db:migrate");
  expect(config.motdCommands.map((row) => row.command)).toEqual(["acme dev", "acme test"]);
});
