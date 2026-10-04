import { afterEach, expect, spyOn, test } from "bun:test";
import { mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { Command, Menu } from "@drkmttr/prelude";
import { platformLibrary } from "../src/internal/ffi.ts";

afterEach(() => {
  process.exitCode = 0;
});

test("parents keep children out of the root menu and dispatch both route forms", async () => {
  const calls: unknown[] = [];
  const menu = Menu.make({
    dispatcher: "acme",
    commands: {
      hl: Command.make({
        description: "Hyperliquid",
        shortcut: "h",
        children: {
          grid: Command.make({
            shortcut: "g",
            motd: 1,
            args: [
              { token: "--config", default: "grids.ts" },
              { token: "--once", boolean: true },
            ],
            run: (args, context) => {
              calls.push([args, context]);
              return 7;
            },
          }),
        },
      }),
    },
  });
  const roots = menu.config.groups.flatMap((group) => group.tasks);
  expect(roots.map((task) => task.name)).toEqual(["hl"]);
  expect(roots[0]!.children?.map((task) => [task.name, task.label])).toEqual([["hl grid", "grid"]]);
  expect(menu.config.motdCommands[0]?.command).toBe("acme hl grid");
  const config = "a trader's $(echo literal) config.ts";
  expect(await menu.dispatch(["hl", "grid", "--config", config])).toBe(7);
  expect(await menu.dispatch(["hl/grid", "--once"])).toBe(7);
  expect(await menu.dispatch(["h", "g", "--once"])).toBe(7);
  expect(calls).toEqual([
    [
      { config, once: false },
      { key: "hl/grid", argv: ["--config", config] },
    ],
    [
      { config: "grids.ts", once: true },
      { key: "hl/grid", argv: ["--once"] },
    ],
    [
      { config: "grids.ts", once: true },
      { key: "hl/grid", argv: ["--once"] },
    ],
  ]);
  const stderr = spyOn(process.stderr, "write").mockImplementation(() => true);
  try {
    expect(await menu.dispatch(["hl", "missing"])).toBe(2);
    expect(calls).toHaveLength(3);
    expect(stderr.mock.calls[0]?.[0]).toContain('unknown subcommand "missing" of hl');
  } finally {
    stderr.mockRestore();
  }
});

test.skipIf(!process.env.PRELUDE_LIB && platformLibrary() === undefined)(
  "native selections return declared children to the TypeScript host",
  async () => {
    let called = "";
    const menu = Menu.make({
      dispatcher: "acme",
      commands: {
        tools: Command.make({
          children: {
            greet: Command.make({
              args: [{ token: "--name", required: true }],
              run: ({ name }) => {
                called = name;
              },
            }),
            status: Command.make({ exec: "exit 3" }),
          },
        }),
      },
    });
    expect(menu.list()).toContain("▸ 2");
    for (const route of [["tools", "greet"], ["tools/greet"]]) {
      const selection = menu.select([...route, "--name", "world"]);
      expect(selection).toMatchObject({ key: "tools/greet", source: "declared" });
      expect(await menu.run(selection!)).toBe(0);
      expect(called).toBe("world");
    }
    expect(await menu.dispatch(["tools", "status"])).toBe(3);
  },
);

test.skipIf(!process.env.PRELUDE_LIB && platformLibrary() === undefined)(
  "declared child routes hide imported scripts without claiming unscoped shortcuts",
  async () => {
    const dir = realpathSync(mkdtempSync(join(tmpdir(), "prelude-subcommands-")));
    try {
      const routes = [
        ["tools/status"],
        ["tools", "status"],
        ["t", "status"],
        ["tools", "s"],
        ["t", "s"],
      ];
      const scripts = Object.fromEntries(routes.map((route) => [route.join(" "), "exit 19"]));
      const packageJson = join(dir, "package.json");
      writeFileSync(
        packageJson,
        JSON.stringify({ scripts: { ...scripts, s: "exit 3", "t/s": "exit 4" } }),
      );
      const calls: string[] = [];
      const menu = Menu.make({
        dispatcher: "acme",
        commands: {
          tools: Command.make({
            shortcut: "t",
            children: {
              status: Command.make({
                shortcut: "s",
                run: (_, context) => {
                  calls.push(context.key);
                  return 7;
                },
              }),
            },
          }),
        },
        scripts: { enable: true, packageJson },
      });

      const text = menu.list(100);
      // `t/s` is the route `t s` (a `/` nests a script name), so it is hidden too.
      for (const key of [...Object.keys(scripts), "t/s"]) {
        expect(text).toContain(`${key}: package.json script hidden by declared command tools`);
      }
      expect(text).not.toContain("exit 19");
      for (const route of routes) {
        expect(menu.select(route)).toMatchObject({ key: "tools/status", source: "declared" });
        expect(await menu.dispatch(route)).toBe(7);
      }
      expect(calls).toEqual(routes.map(() => "tools/status"));
      expect(menu.select(["s"])).toMatchObject({ key: "s", source: "scripts" });
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  },
);

test("rejects invalid and ambiguous submenu declarations", () => {
  const leaf = Command.make({ exec: "true" });
  const parent = Command.make({ children: { leaf } });
  expect(() => Command.make({ children: {} })).toThrow("at least one child");
  expect(() => Command.make({ children: { nested: parent } } as never)).toThrow("must be a leaf");
  expect(() => Command.make({ children: { leaf }, run: () => {} } as never)).toThrow("exactly one");
  expect(() => Command.make({ children: { leaf }, args: [] } as never)).toThrow(
    "cannot declare arguments",
  );
  expect(() => Menu.make({ commands: { tools: parent, "tools/leaf": leaf } })).toThrow(
    "duplicate mounted command key",
  );
  expect(() =>
    Menu.make({ commands: { tools: Command.make({ children: { "bad name": leaf } }) } }),
  ).toThrow("command key");
});
