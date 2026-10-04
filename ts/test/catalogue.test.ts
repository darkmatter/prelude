import { describe, expect, test } from "bun:test";

import { Command, Menu, Motd } from "@drkmttr/prelude";

const shell = (exec: string, extra: Record<string, unknown> = {}) => Command.make({ exec, ...extra });
const parent = (extra: Record<string, unknown> = {}) => Command.make(extra);
const config = (commands: Menu.Options["commands"], groupOrder?: string[]) =>
  Menu.make({ commands, groupOrder, dispatcher: "acme" }).config;
const titles = (commands: Menu.Options["commands"], groupOrder?: string[]) =>
  config(commands, groupOrder).groups.map((group) => [group.title, group.tasks.map((task) => task.label)]);
/** Each task's name, with its subcommands' names nested beside it. */
const tree = (tasks: readonly Menu.Task[]): unknown[] =>
  tasks.map((task) => (task.children === undefined ? task.name : [task.name, tree(task.children)]));
const everyTask = (tasks: readonly Menu.Task[]): Menu.Task[] =>
  tasks.flatMap((task) => [task, ...everyTask(task.children ?? [])]);

describe("catalogue identity", () => {
  test("keys are never parsed for a group, and : is part of the name", () => {
    expect(
      titles({
        build: shell("make"),
        "go:test": shell("go test ./..."),
        "test:unit:watch": shell("bun test --watch"),
        lint: shell("eslint .", { group: "quality" }),
      }),
    ).toEqual([
      ["", ["build", "go:test", "test:unit:watch"]],
      ["quality", ["lint"]],
    ]);
  });

  test("ungrouped commands come first, then prelude's group, then groupOrder, then the rest alphabetically", () => {
    expect(
      titles(
        {
          "z:one": shell("z1", { group: "z" }),
          "a:one": shell("a1", { group: "a" }),
          up: shell("up", { group: "db" }),
          deploy: shell("deploy"),
          // Prelude's own docs command lists under prelude without saying so.
          docs: Command.make({ run: () => {} }),
        },
        ["db"],
      ),
    ).toEqual([
      ["", ["deploy"]],
      ["prelude", ["docs"]],
      ["db", ["up"]],
      ["a", ["a:one"]],
      ["z", ["z:one"]],
    ]);
  });

  test("rejects keys, groups, and shortcuts the dispatcher cannot resolve", () => {
    for (const key of ["", "has  two", "db/", "/db", "db//migrate", "db /migrate", "tab\tkey", "a,b"]) {
      expect(() => titles({ [key]: shell("x") })).toThrow(`command key "${key}" must be words of letters, digits`);
    }
    expect(() => titles({ "db/migrate": shell("a"), "db migrate": shell("b") })).toThrow(
      'these keys name the same command: "db migrate", "db/migrate"',
    );
    expect(() => titles({ "db migrate": shell("a", { group: "data" }) })).toThrow(
      '"db migrate" is a subcommand of "db"; set group on "db" instead',
    );
    expect(() => titles({ db: parent({ description: "database tasks" }) })).toThrow(
      'commands["db"] has neither `run` nor `exec`',
    );
    expect(() => titles({ a: shell("a", { shortcut: "b" }), b: shell("b") })).toThrow("collides with a command name");
    // A parent only its subcommands imply has a name too.
    expect(() => titles({ a: shell("a", { shortcut: "db" }), "db migrate": shell("m") })).toThrow(
      "collides with a command name",
    );
    expect(() => titles({ a: shell("a", { shortcut: "x" }), b: shell("b", { shortcut: "x" }) })).toThrow(
      "used by both a and b",
    );
    expect(() => titles({ t: shell("bun test"), "test unit": shell("bun test") })).toThrow(
      "duplicate canonical command invocation(s): bun test",
    );
    expect(() => titles({ a: { exec: "a" } as never })).toThrow("create it with Command.make()");
  });
});

describe("subcommands", () => {
  test("a space or slash nests a command under its parent, to any depth", () => {
    const { groups } = config({
      "db migrate": shell("drizzle-kit migrate"),
      "db/seed": shell("bun run seed"),
      "db seed/users": shell("bun run seed users"),
      "ops/deploy/web": shell("vercel deploy"),
      "ops deploy api": shell("fly deploy"),
      lint: shell("eslint .", { group: "quality" }),
    });
    expect(groups.map((group) => [group.title, tree(group.tasks)])).toEqual([
      [
        "",
        [
          ["db", ["db migrate", ["db seed", ["db seed users"]]]],
          ["ops", [["ops deploy", ["ops deploy api", "ops deploy web"]]]],
        ],
      ],
      ["quality", ["lint"]],
    ]);
  });

  test("a parent no key declares only opens its subcommands, and leaves carry no children", () => {
    const [ops] = config({ "ops/deploy/web": shell("vercel deploy") }).groups.flatMap((group) => group.tasks);
    const blank = { key: "", usage: "", details: "", examples: [], args: [] };
    expect(ops).toStrictEqual({
      name: "ops",
      label: "ops",
      run: "",
      command: "acme ops",
      description: "1 subcommand",
      ...blank,
      children: [
        {
          name: "ops deploy",
          label: "deploy",
          run: "",
          command: "acme ops deploy",
          description: "1 subcommand",
          ...blank,
          children: [
            {
              name: "ops deploy web",
              label: "web",
              run: "vercel deploy",
              command: "acme ops deploy web",
              description: "",
              ...blank,
            },
          ],
        },
      ],
    });
  });

  test("a parent without run or exec is a container; one with either runs itself", () => {
    const { groups } = config(
      {
        db: parent({ description: "database tasks", group: "data" }),
        "db migrate": shell("drizzle-kit migrate"),
        tools: parent({ group: "data" }),
        "tools fmt": shell("nix fmt"),
        cache: shell("nix-store --gc"),
        "cache stats": shell("du -sh /nix/store"),
        serve: Command.make({ run: () => {} }),
        "serve docs": Command.make({ run: () => {} }),
        "ui/build": shell("vite build"),
        "ui/test": shell("vitest"),
      },
      ["data"],
    );
    expect(groups.map((group) => [group.title, group.tasks.map((task) => [task.name, task.run, task.description])])).toEqual([
      [
        "",
        [
          ["cache", "nix-store --gc", ""],
          ["serve", "acme serve", ""],
          ["ui", "", "2 subcommands"],
        ],
      ],
      [
        "data",
        [
          ["db", "", "database tasks"],
          ["tools", "", "1 subcommand"],
        ],
      ],
    ]);
  });

  test("every node runs through the dispatcher by its words, since an app has no PATH wrappers", () => {
    const commands = {
      // In the devshell, a declared root without `:` runs bare from PATH, and
      // the rest go through `x`; here all of them take the dispatcher.
      db: parent({ description: "database tasks" }),
      "db/migrate": shell("drizzle-kit migrate", { motd: 2 }),
      "go test": shell("go test ./..."),
      "go:vet": shell("go vet ./..."),
      "go:vet strict": shell("go vet -strict ./...", { motd: 1 }),
      tools: parent({ motd: 3 }),
      "tools fmt": shell("nix fmt"),
    };
    const { groups, motdCommands } = config(commands);
    expect(everyTask(groups.flatMap((group) => group.tasks)).map((task) => [task.name, task.command])).toEqual([
      ["db", "acme db"],
      ["db migrate", "acme db migrate"],
      ["go", "acme go"],
      ["go test", "acme go test"],
      ["go:vet", "acme go:vet"],
      ["go:vet strict", "acme go:vet strict"],
      ["tools", "acme tools"],
      ["tools fmt", "acme tools fmt"],
    ]);
    // Getting Started lists commands at any depth, by position, then name.
    const started = [
      { name: "go:vet strict", command: "acme go:vet strict", description: "" },
      { name: "db migrate", command: "acme db migrate", description: "" },
      { name: "tools", command: "acme tools", description: "1 subcommand" },
    ];
    expect(motdCommands).toEqual(started);
    expect(Motd.make({ commands, dispatcher: "acme" }).config.commands).toEqual(
      started.map(({ command, description }) => ({ command, description })),
    );
  });
});
