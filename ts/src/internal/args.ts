import type * as Args from "../Args.ts";

const namePattern = /^[A-Za-z][A-Za-z0-9]*(-[A-Za-z0-9]+)*$/;

function bareName(token: string): string {
  if (token.startsWith("--")) return token.slice(2);
  if (token.startsWith("<") && token.endsWith(">")) return token.slice(1, -1);
  return token;
}

/** The runtime twin of Args.Name: `--dry-run` → `dryRun`. */
export function argName(token: string): string {
  return bareName(token).replace(/-([A-Za-z0-9])/g, (_, next: string) => next.toUpperCase());
}

/** Rejects declarations the parser or the picker could not honor. */
export function validate(specs: readonly Args.Spec[], where: string): void {
  const names = new Map<string, string>();
  for (const spec of specs) {
    const { token } = spec;
    if (!namePattern.test(bareName(token))) {
      throw new Error(`${where}: argument token "${token}" must be --name, <name>, or NAME (letters, digits, single dashes)`);
    }
    if (spec.boolean && !token.startsWith("--")) {
      throw new Error(`${where}: boolean argument "${token}" must be a --flag`);
    }
    const name = argName(token);
    const clash = names.get(name);
    if (clash !== undefined) {
      throw new Error(`${where}: arguments "${clash}" and "${token}" would both be args.${name}`);
    }
    names.set(name, token);
    if (spec.default !== undefined && spec.options && !spec.options.includes(String(spec.default))) {
      throw new Error(`${where}: default for ${token} must be one of ${spec.options.join(", ")}`);
    }
  }
}
