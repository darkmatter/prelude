import { describe, expect, test } from "bun:test";

import { Args, Command } from "@drkmttr/prelude";

const serve = [
  { token: "--port", type: "number", default: 3000 },
  { token: "--host", options: ["127.0.0.1", "0.0.0.0"] },
  { token: "--open-browser", boolean: true },
  { token: "<target>" },
] as const;

describe("Args.parse", () => {
  test("parses options, flags, and positionals into typed camelCase values", () => {
    expect(Args.parse(serve, ["--port", "8080", "--host=0.0.0.0", "--open-browser", "web"])).toEqual({
      port: 8080,
      host: "0.0.0.0",
      openBrowser: true,
      target: "web",
    });
  });

  test("applies defaults and leaves optional values unset", () => {
    expect(Args.parse(serve, [])).toEqual({ port: 3000, openBrowser: false });
  });

  test("treats words after -- as positionals", () => {
    expect(Args.parse(serve, ["--", "--port"])).toEqual({ port: 3000, openBrowser: false, target: "--port" });
  });

  test("reports mistakes as ParseError", () => {
    const cases: [readonly string[], string][] = [
      [["--nope"], "unknown option --nope"],
      [["--port"], "--port needs a value"],
      [["--port", "abc"], '--port expects a number; got "abc"'],
      [["--host", "example.com"], '--host must be one of 127.0.0.1, 0.0.0.0; got "example.com"'],
      [["--open-browser=yes"], "--open-browser takes no value"],
      [["web", "extra"], 'unexpected argument "extra"'],
    ];
    for (const [argv, message] of cases) {
      expect(() => Args.parse(serve, argv)).toThrow(new Args.ParseError(message));
    }
    expect(() => Args.parse([{ token: "--env", required: true }], [])).toThrow("missing required argument --env");
  });
});

describe("Args.split", () => {
  test("splits argument text like a shell, without expansion", () => {
    expect(Args.split(`--msg "hello world" --name 'it''s' plain\\ word "a\\"b" $HOME`)).toEqual([
      "--msg",
      "hello world",
      "--name",
      "its",
      "plain word",
      'a"b',
      "$HOME",
    ]);
    expect(Args.split("   ")).toEqual([]);
    expect(Args.split(`''`)).toEqual([""]);
  });

  test("rejects an unterminated quote", () => {
    expect(() => Args.split(`--msg "oops`)).toThrow(Args.ParseError);
  });
});

describe("Command.make", () => {
  test("needs exactly one of run or exec", () => {
    expect(() => Command.make({} as never)).toThrow("exactly one of `run`");
    expect(() => Command.make({ exec: "true", run: () => {} } as never)).toThrow("exactly one of `run`");
  });

  test("rejects argument tokens the parser cannot honor", () => {
    expect(() => Command.make({ exec: "x", args: [{ token: "-p" }] })).toThrow('argument token "-p"');
    expect(() => Command.make({ exec: "x", args: [{ token: "<file>", boolean: true }] })).toThrow("must be a --flag");
    expect(() => Command.make({ exec: "x", args: [{ token: "--dry-run" }, { token: "<dry-run>" }] })).toThrow(
      "would both be args.dryRun",
    );
    expect(() => Command.make({ exec: "x", args: [{ token: "--mode", options: ["a"], default: "b" }] })).toThrow(
      "default for --mode",
    );
  });
});
