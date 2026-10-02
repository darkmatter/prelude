import { constants } from "node:os";
import { basename, delimiter, isAbsolute, relative } from "node:path";

import type * as Palette from "../Palette.ts";
import { paint } from "./palette.ts";

/**
 * How people run this app from a shell: a compiled executable's name, or the
 * script's path relative to the working directory when it lives below it.
 */
export function defaultDispatcher(): string {
  const script = process.argv[1];
  if (script === undefined || script.startsWith("/$bunfs/")) return basename(process.execPath);
  const path = relative(process.cwd(), script);
  return path.startsWith("..") || isAbsolute(path) ? basename(script) : `./${path}`;
}

/**
 * Runs shell text with bash (sh when bash is missing) on this terminal and
 * resolves with its exit status, 128 + n for a signal. It runs in `dir` when
 * given, with `pathPrefix` ahead of PATH. Terminal signals (Ctrl+C, Ctrl+\)
 * reach the child through the shared process group, so this process ignores
 * them and outlives it; SIGTERM and SIGHUP sent here are forwarded.
 */
export async function runShell(
  command: string,
  { dir, pathPrefix = [] }: { dir?: string; pathPrefix?: readonly string[] } = {},
): Promise<number> {
  const shell = Bun.which("bash") ?? Bun.which("sh") ?? "sh";
  const env =
    pathPrefix.length === 0
      ? process.env
      : { ...process.env, PATH: [...pathPrefix, ...(process.env.PATH ? [process.env.PATH] : [])].join(delimiter) };
  const child = Bun.spawn([shell, "-c", command], { cwd: dir, env, stdio: ["inherit", "inherit", "inherit"] });
  const ignore = () => {};
  const forward = (signal: NodeJS.Signals) => child.kill(signal);
  process.on("SIGINT", ignore);
  process.on("SIGQUIT", ignore);
  process.on("SIGTERM", forward);
  process.on("SIGHUP", forward);
  try {
    const code = await child.exited;
    const signal = child.signalCode;
    return signal ? 128 + (constants.signals[signal] ?? 0) : code;
  } finally {
    process.off("SIGINT", ignore);
    process.off("SIGQUIT", ignore);
    process.off("SIGTERM", forward);
    process.off("SIGHUP", forward);
  }
}

/** A `run` result as an exit status: a returned integer, else success. */
export function exitStatus(result: unknown): number {
  return typeof result === "number" && Number.isInteger(result) ? result : 0;
}

/** Announces what is about to run, as the Go menu does before it execs. */
export function announce(palette: Palette.Palette, command: string): void {
  process.stdout.write(`\n${paint(palette.accent, "$ ")}${paint(palette.fg, command)}\n\n`);
}

/** Reports a failure on stderr in the palette's error color. */
export function complain(palette: Palette.Palette, message: string): void {
  process.stderr.write(`${paint(palette.error, message, process.stderr)}\n`);
}
