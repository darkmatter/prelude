/**
 * libprelude inside the npm package: one library per platform under
 * lib/<os>-<cpu>/, all shipped in `@drkmttr/prelude`, so an install needs
 * nothing else. src/internal/ffi.ts loads this machine's.
 *
 *   bun scripts/npm-package.ts library [os-cpu]   build one platform's library into lib/
 *   bun scripts/npm-package.ts check              fail unless every platform's library is in lib/
 *   bun scripts/npm-package.ts smoke              install the packed package and load the menu
 *
 * The library needs cgo, so each platform builds on its own OS (the publish
 * workflow's matrix); `library` defaults to this machine's.
 */
import { $ } from "bun";
import { mkdir, mkdtemp, readdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

process.chdir(new URL("../", import.meta.url).pathname);

/** `os` and `cpu` are Node's `process.platform` and `process.arch`, which ffi.ts names the directory from. */
const platforms = [
  { os: "darwin", cpu: "arm64", goarch: "arm64" },
  { os: "darwin", cpu: "x64", goarch: "amd64" },
  { os: "linux", cpu: "arm64", goarch: "arm64" },
  { os: "linux", cpu: "x64", goarch: "amd64" },
] as const;

type Platform = (typeof platforms)[number];

const id = ({ os, cpu }: Platform) => `${os}-${cpu}`;
const libraryPath = (platform: Platform) =>
  `lib/${id(platform)}/libprelude.${platform.os === "darwin" ? "dylib" : "so"}`;

const fail = (message: string): never => {
  console.error(message);
  process.exit(2);
};

const platformNamed = (name: string) =>
  platforms.find((platform) => id(platform) === name) ??
  fail(`unknown platform ${name}; expected one of ${platforms.map(id).join(", ")}`);

const library = async (name = `${process.platform}-${process.arch}`) => {
  const platform = platformNamed(name);
  if (platform.os !== process.platform) fail(`build the ${name} library on ${platform.os}; cgo does not cross OSes here`);
  const env: Record<string, string> = { CGO_ENABLED: "1", GOOS: platform.os, GOARCH: platform.goarch };
  // A Mac builds the other architecture's library by pointing clang at it.
  if (platform.cpu !== process.arch) {
    if (platform.os !== "darwin") fail(`build the ${name} library on a ${platform.cpu} machine`);
    env.CC = `clang -arch ${platform.cpu === "x64" ? "x86_64" : "arm64"}`;
  }
  const directory = `lib/${id(platform)}`;
  const output = `${process.cwd()}/${libraryPath(platform)}`;
  await rm(directory, { recursive: true, force: true });
  await mkdir(directory, { recursive: true });
  await $`go build -C ../src -buildmode=c-shared -trimpath -ldflags ${"-s -w"} -o ${output} ./cmd/libprelude`.env({
    ...process.env,
    ...env,
  });
  // The C header serves C callers; the package ships only the library.
  await rm(`${directory}/libprelude.h`, { force: true });
};

const check = async () => {
  const missing: string[] = [];
  for (const platform of platforms) {
    if (!(await Bun.file(libraryPath(platform)).exists())) missing.push(libraryPath(platform));
  }
  if (missing.length > 0) {
    fail(`missing ${missing.join(", ")}: build each with \`bun scripts/npm-package.ts library <os-cpu>\` on its OS`);
  }
};

// Loads the menu from the installed package, with PRELUDE_LIB unset, so the
// library can only come from the package's own lib/.
const smokeScript = `
import { Command, Menu } from "@drkmttr/prelude";
const menu = Menu.make({ project: "smoke", dispatcher: "smoke", commands: { hello: Command.make({ exec: "true" }) } });
if (!menu.list(60).includes("run smoke to pick a task interactively")) throw new Error("unexpected --list output");
console.log("@drkmttr/prelude loaded its bundled libprelude");
`;

/** Installs the packed package into a scratch project, as a user's install would, and loads the menu there. */
const smoke = async () => {
  const project = await mkdtemp(join(tmpdir(), "prelude-smoke-"));
  try {
    await $`npm pack --silent --pack-destination ${project}`.quiet();
    await Bun.write(join(project, "package.json"), `{ "private": true }\n`);
    await Bun.write(join(project, "smoke.ts"), smokeScript);
    const tarball = (await readdir(project)).find((file) => file.endsWith(".tgz"));
    await $`npm install --no-audit --no-fund --silent ./${tarball}`.cwd(project).quiet();
    const { PRELUDE_LIB: _, ...env } = process.env;
    await $`bun smoke.ts`.cwd(project).env(env);
  } finally {
    await rm(project, { recursive: true, force: true });
  }
};

const [command, platform] = Bun.argv.slice(2);
if (command === "library") await library(platform);
else if (command === "check") await check();
else if (command === "smoke") await smoke();
else fail("usage: bun scripts/npm-package.ts library [os-cpu] | check | smoke");
