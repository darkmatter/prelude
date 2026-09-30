import { describe, expect, test } from "bun:test";

import { Command, Menu } from "@drkmttr/prelude";

const shell = (exec: string, extra: Record<string, unknown> = {}) => Command.make({ exec, ...extra });
const titles = (commands: Menu.Options["commands"], groupOrder?: string[]) =>
  Menu.make({ commands, groupOrder, dispatcher: "acme" }).config.groups.map((group) => [
    group.title,
    group.tasks.map((task) => task.label),
  ]);

describe("catalogue identity", () => {
  test("the first colon picks the group and the rest is the label", () => {
    expect(
      titles({
        build: shell("make"),
        "go:test": shell("go test ./..."),
        "test:unit:watch": shell("bun test --watch"),
        lint: shell("eslint .", { group: "quality" }),
      }),
    ).toEqual([
      ["develop", ["build"]],
      ["go", ["test"]],
      ["quality", ["lint"]],
      ["test", ["unit:watch"]],
    ]);
  });

  test("prelude's group comes first, then groupOrder, then the rest alphabetically", () => {
    expect(
      titles(
        {
          "z:one": shell("z1"),
          "a:one": shell("a1"),
          "db:up": shell("up"),
          docs: Command.make({ group: "prelude", run: () => {} }),
        },
        ["db"],
      ).map(([title]) => title),
    ).toEqual(["prelude", "db", "a", "z"]);
  });

  test("rejects keys and shortcuts the dispatcher cannot resolve", () => {
    expect(() => titles({ "has space": shell("x") })).toThrow('command key "has space"');
    expect(() => titles({ "go:": shell("x") })).toThrow("non-empty colon-separated segments");
    expect(() => titles({ a: shell("a", { shortcut: "b" }), b: shell("b") })).toThrow("collides with a command key");
    expect(() => titles({ a: shell("a", { shortcut: "x" }), b: shell("b", { shortcut: "x" }) })).toThrow(
      "used by both a and b",
    );
    expect(() => titles({ t: shell("bun test"), test: shell("bun test") })).toThrow(
      "duplicate canonical command invocation(s): bun test",
    );
    expect(() => titles({ a: { exec: "a" } as never })).toThrow("create it with Command.make()");
  });
});
