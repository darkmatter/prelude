import { describe, expect, test } from "bun:test";

import { Command, Docs, Menu, Motd } from "@drkmttr/prelude";

// These drive the real Go surfaces through bun:ffi. The flake's ts-api check
// and the repo devshell set PRELUDE_LIB; without it they are skipped. Only
// non-interactive calls run here: the picker and viewer need a terminal.

const plain = (text: string) => text.replace(/\x1b\[[0-9;:?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, "");

describe.skipIf(!process.env.PRELUDE_LIB)("libprelude", () => {
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
    expect(menu.select(["lint", "--fix"])).toEqual({ key: "lint", line: "--fix", shell: "eslint . --fix" });
    expect(() => menu.select(["nope"])).toThrow('unknown command "nope"');
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
