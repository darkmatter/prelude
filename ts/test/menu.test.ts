import { afterEach, describe, expect, spyOn, test } from "bun:test";
import { mkdirSync, mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

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

describe("run", () => {
  const quietly = async (run: () => Promise<number>) => {
    // Shell selections announce themselves on stdout first.
    const stdout = spyOn(process.stdout, "write").mockImplementation(() => true);
    try {
      return await run();
    } finally {
      stdout.mockRestore();
    }
  };

  test("runs shell text in the selection's directory with its PATH prefix", async () => {
    const dir = realpathSync(mkdtempSync(join(tmpdir(), "prelude-run-")));
    try {
      const bin = join(dir, "bin");
      mkdirSync(bin);
      writeFileSync(join(bin, "greet"), '#!/bin/sh\necho "hi from $(pwd)" > greeting\n', { mode: 0o755 });
      const menu = Menu.make({ dispatcher: "acme", commands: { noop: Command.make({ exec: "true" }) } });

      const selection = { key: "greet", line: "", shell: "greet", source: "just", dir, pathPrefix: [bin] } as const;
      expect(await quietly(() => menu.run(selection))).toBe(0);
      expect(readFileSync(join(dir, "greeting"), "utf8")).toBe(`hi from ${dir}\n`);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  test("runs an import as shell text even when one of the app's functions shares its key", async () => {
    let ran = false;
    const menu = Menu.make({
      dispatcher: "acme",
      commands: { test: Command.make({ run: () => void (ran = true) }) },
    });
    expect(await quietly(() => menu.run({ key: "test", line: "", shell: "exit 3", source: "just" }))).toBe(3);
    expect(ran).toBe(false);
    expect(await quietly(() => menu.run({ key: "test", line: "", shell: "acme test", source: "declared" }))).toBe(0);
    expect(ran).toBe(true);
  });
});
