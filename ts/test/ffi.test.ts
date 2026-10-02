import { describe, expect, test } from "bun:test";
import { mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";

import { Command, Docs, Menu, Motd } from "@drkmttr/prelude";

import { platformLibrary } from "../src/internal/ffi.ts";

// These drive the real Go surfaces through bun:ffi. The flake's ts-api check
// and the repo devshell set PRELUDE_LIB, and CI builds the library into lib/;
// with neither they are skipped. Only non-interactive calls run here: the
// picker and viewer need a terminal.

/** Every directory above dir, nearest first. */
const parents = (dir: string): string[] => (dirname(dir) === dir ? [] : [dirname(dir), ...parents(dirname(dir))]);

const plain = (text: string) => text.replace(/\x1b\[[0-9;:?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, "");

describe.skipIf(!process.env.PRELUDE_LIB && platformLibrary() === undefined)("libprelude", () => {
  const menu = Menu.make({
    project: "acme",
    dispatcher: "acme",
    commands: {
      lint: Command.make({ exec: "eslint .", description: "lint the project" }),
      serve: Command.make({ description: "run the dev server", run: () => {} }),
    },
  });

  test("lists the catalogue the way `x --list` does, naming this app", () => {
    const text = plain(menu.list(60));
    expect(text).toContain("lint the project");
    expect(text).toContain("run the dev server");
    expect(text).toContain("run acme to pick a task interactively");
  });

  test("resolves typed words like x without opening the picker", () => {
    expect(menu.select(["lint", "--fix"])).toEqual({ key: "lint", line: "--fix", shell: "eslint . --fix", source: "declared" });
    expect(() => menu.select(["nope"])).toThrow('unknown command "nope"');
  });

  test("imports package.json scripts that run as written, where they live", () => {
    const dir = realpathSync(mkdtempSync(join(tmpdir(), "prelude-scripts-")));
    try {
      const packageJson = join(dir, "package.json");
      writeFileSync(packageJson, JSON.stringify({ scripts: { build: "tsc && vite build", lint: "eslint" } }));
      const withScripts = Menu.make({
        dispatcher: "acme",
        commands: { lint: Command.make({ exec: "eslint .", description: "lint the project" }) },
        scripts: { enable: true, packageJson },
      });

      expect(withScripts.select(["build", "--watch"])).toEqual({
        key: "build",
        line: "--watch",
        shell: "tsc && vite build --watch",
        source: "scripts",
        dir,
        pathPrefix: [join(dir, "node_modules/.bin"), ...parents(dir).map((parent) => join(parent, "node_modules/.bin"))],
      });
      const text = plain(withScripts.list(80));
      expect(text).toContain("tsc && vite build");
      // The app's own lint keeps its name; the script is reported, not dropped.
      expect(text).toContain("lint: package.json script hidden by declared command");
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  test("renders the MOTD with TypeScript checks and shell probes", async () => {
    const motd = Motd.make({
      project: "acme",
      dispatcher: "acme",
      header: {
        status: {
          // An empty ok text shows the check's output instead, as in Nix.
          api: { label: "api", ok: "", check: async () => ({ ok: true, output: "reachable" }) },
          db: { label: "db", fail: "down", check: () => false },
        },
      },
      env: [
        { label: "runtime", probe: () => `bun ${Bun.version}` },
        { label: "shell", probe: "echo from-bash" },
      ],
    });
    const banner = plain(await motd.render({ width: 90, height: 30 }));
    for (const expected of ["reachable", "down", `bun ${Bun.version}`, "from-bash"]) {
      expect(banner).toContain(expected);
    }
  });

  test("renders docs pages by number", () => {
    const pages = [
      { title: "Guide", markdown: "# Guide\n\nHello from the guide." },
      { title: "More", children: [{ title: "Deploy", markdown: "Ship with care." }] },
    ];
    const docs = Docs.make({ pages });
    expect(docs.pages()).toBe(2);
    expect(plain(docs.render({ page: 2, width: 60 }))).toContain("Ship with care.");
    expect(() => docs.render({ page: 3 })).toThrow("out of range");
  });
});
