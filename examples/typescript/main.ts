#!/usr/bin/env bun
// The app. Each command is mounted under the name people type, and the first
// colon groups it in the menu (`db:migrate` goes under "db"). Configuring a
// MOTD and docs adds the built-in `motd` and `docs` commands.
//
//   ./main.ts                   pick a command
//   ./main.ts dev --port 8080   run one directly
//   ./main.ts --list            print them all
import { Prelude } from "@drkmttr/prelude";

import { migrateCommand, seedCommand } from "./db.ts";
import guide from "./guide.md" with { type: "text" };
import { preludeCommand as dev } from "./server.ts";

const app = Prelude.make({
  project: "acme",
  theme: "minted",

  commands: {
    dev,
    "db:migrate": migrateCommand,
    "db:seed": seedCommand,
  },

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
