// Two kinds of command: one runs a TypeScript function, the other runs shell
// text, for work another tool already owns.
import { Command } from "@drkmttr/prelude";

const migrations = ["0001_users", "0002_projects", "0003_invites"];

export function migrate({ dryRun }: { dryRun: boolean }) {
  for (const migration of migrations) {
    console.log(`${dryRun ? "would apply" : "applied"} ${migration}`);
  }
}

export const migrateCommand = Command.make({
  description: "apply pending migrations",
  args: [{ token: "--dry-run", description: "show the plan without applying it", boolean: true }],
  run: (args) => migrate({ dryRun: args.dryRun }),
  motd: 2,
});

export const seedCommand = Command.make({
  description: "load sample rows",
  exec: "echo 'seeded 3 sample rows'",
});
