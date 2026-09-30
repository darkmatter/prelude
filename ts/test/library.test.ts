import { afterAll, expect, test } from "bun:test";
import { suffix } from "bun:ffi";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

import { platformLibrary } from "../src/internal/ffi.ts";

// The package ships lib/<os>-<cpu>/libprelude.<suffix> for every supported
// platform; resolution must pick this machine's from the package root.
const root = mkdtempSync(join(tmpdir(), "prelude-package-"));
afterAll(() => rmSync(root, { recursive: true, force: true }));

test("finds the library the package ships for this machine", () => {
  const library = join(root, "lib", `${process.platform}-${process.arch}`, `libprelude.${suffix}`);
  mkdirSync(join(library, ".."), { recursive: true });
  writeFileSync(library, "");
  expect(platformLibrary(pathToFileURL(`${root}/`))).toBe(library);
});

test("finds nothing when the package has no library for this machine", () => {
  expect(platformLibrary(pathToFileURL(`${join(root, "empty")}/`))).toBeUndefined();
});
