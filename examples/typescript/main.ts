#!/usr/bin/env bun
// The app. Each command is mounted under the words people type: a `/` (or a
// space) makes a subcommand, so `db/migrate` runs as `db migrate` and the
// menu shows one `db` row that opens it. The scripts in package.json join the
// menu too, and configuring a MOTD and docs adds the built-in `motd` and
// `docs` commands.
//
//   ./main.ts                   pick a command
//   ./main.ts dev --port 8080   run one directly
//   ./main.ts build             run a package.json script, from this folder
//   ./main.ts --list            print them all
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { Prelude } from "@drkmttr/prelude";

import { migrateCommand, seedCommand } from "./db.ts";
import guide from "./guide.md" with { type: "text" };
import { preludeCommand as dev } from "./server.ts";

// The scripts next to this file. A single-file build of the app has no
// package.json beside it, so it leaves them out.
const packageJson = fileURLToPath(new URL("./package.json", import.meta.url));

const app = Prelude.make({
  project: "acme",
  theme: "minted",

  commands: {
    dev,
    "db/migrate": migrateCommand,
    "db/seed": seedCommand,
  },

  menu: { scripts: { enable: existsSync(packageJson), packageJson } },

  motd: {
    header: {
      tagline: { text: "everything you need to build, test & ship" },
      // A check can be a function. With `ok` empty, the badge shows its result.
      status: { runtime: { label: "bun", ok: "", check: () => Bun.version } },
    },
    description: { text: "An example prelude app written in TypeScript." },
  },

  docs: { pages: [{ markdown: guide }] },
});

await app.main();
