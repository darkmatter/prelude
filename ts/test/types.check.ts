// Type-level tests: `bun run typecheck` (tsc) checks this file; bun test does
// not run it. Each @ts-expect-error marks a mistake the types must catch.
import { Command, Menu } from "@drkmttr/prelude";

const serve = (_port: number, _host?: "127.0.0.1" | "0.0.0.0", _open?: boolean, _target?: string) => {};

// `run` may come before `args`, and no `as const` is needed.
export const preludeCommand = Command.make({
  run: (args) => serve(args.port, args.host, args.openBrowser, args.target),
  description: "run the dev server",
  args: [
    { token: "--port", description: "port to run on", type: "number", default: 3000 },
    { token: "--host", options: ["127.0.0.1", "0.0.0.0"] },
    { token: "--open-browser", boolean: true },
    { token: "<target>" },
  ],
});

Command.make({
  // @ts-expect-error --port has no default and is not required, so it may be undefined.
  run: (args) => serve(args.port),
  args: [{ token: "--port", type: "number" }],
});

Command.make({
  // @ts-expect-error there is no --prot.
  run: (args) => serve(args.prot),
  args: [{ token: "--port", type: "number", required: true }],
});

// @ts-expect-error a command runs a function or shell text, not both.
Command.make({ exec: "eslint .", run: () => {} });

// Commands with different argument lists share one commands record.
Menu.make({ commands: { dev: preludeCommand, lint: Command.make({ exec: "eslint ." }) } });

const tools = Command.make({ children: { serve: preludeCommand } });
Menu.make({ commands: { tools } });
// @ts-expect-error a parent opens its children and cannot also run a function.
Command.make({ children: { serve: preludeCommand }, run: () => {} });
// @ts-expect-error parent arguments belong on the child command.
Command.make({ children: { serve: preludeCommand }, args: [{ token: "--port" }] });
// @ts-expect-error the native picker supports one submenu level.
Command.make({ children: { nested: tools } });
