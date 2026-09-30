import { afterEach, describe, expect, spyOn, test } from "bun:test";

import { Command, Menu } from "@drkmttr/prelude";

// dispatch() sets process.exitCode; keep it from failing the test run.
afterEach(() => {
  process.exitCode = 0;
});

describe("dispatch", () => {
  test("runs a function command with parsed, typed arguments", async () => {
    const calls: unknown[] = [];
    const menu = Menu.make({
      dispatcher: "acme",
      commands: {
        serve: Command.make({
          shortcut: "s",
          args: [{ token: "--port", type: "number", default: 3000 }, { token: "<target>" }],
          run: (args, context) => {
            calls.push([args, context]);
            return 7;
          },
        }),
      },
    });

    expect(await menu.dispatch(["serve", "--port", "8080", "web"])).toBe(7);
    expect(process.exitCode).toBe(7);
    // Shortcuts resolve too, and `--` ends option parsing.
    expect(await menu.dispatch(["s", "--", "--literal"])).toBe(7);
    expect(calls).toEqual([
      [{ port: 8080, target: "web" }, { key: "serve", argv: ["--port", "8080", "web"] }],
      [{ port: 3000, target: "--literal" }, { key: "serve", argv: ["--", "--literal"] }],
    ]);
  });

  test("reports argument mistakes before anything runs, with status 2", async () => {
    let ran = false;
    const stderr = spyOn(process.stderr, "write").mockImplementation(() => true);
    try {
      const menu = Menu.make({
        dispatcher: "acme",
        commands: {
          serve: Command.make({
            args: [{ token: "--port", type: "number", required: true }],
            run: () => {
              ran = true;
            },
          }),
        },
      });
      expect(await menu.dispatch(["serve", "--port", "x"])).toBe(2);
      expect(ran).toBe(false);
      expect(String(stderr.mock.calls[0]?.[0])).toContain('acme: --port expects a number; got "x"');
    } finally {
      stderr.mockRestore();
    }
  });

  test("treats a non-number return as success", async () => {
    const menu = Menu.make({ dispatcher: "acme", commands: { ok: Command.make({ run: async () => "done" }) } });
    expect(await menu.dispatch(["ok"])).toBe(0);
  });
});
