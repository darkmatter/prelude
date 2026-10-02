import { CString, dlopen, FFIType, ptr, suffix } from "bun:ffi";
import { existsSync } from "node:fs";
import { fileURLToPath } from "node:url";

// The C ABI of src/cmd/libprelude: each operation takes one JSON request and
// returns one JSON reply, {"ok": …} or {"error": "…"}, released with
// prelude_free. Bump abiVersion together with the Go constant.
const abiVersion = 2;

const symbols = {
  prelude_abi_version: { args: [], returns: FFIType.i32 },
  prelude_free: { args: [FFIType.ptr], returns: FFIType.void },
  prelude_menu_select: { args: [FFIType.ptr], returns: FFIType.ptr },
  prelude_menu_list: { args: [FFIType.ptr], returns: FFIType.ptr },
  prelude_motd_render: { args: [FFIType.ptr], returns: FFIType.ptr },
  prelude_docs_render: { args: [FFIType.ptr], returns: FFIType.ptr },
  prelude_docs_open: { args: [FFIType.ptr], returns: FFIType.ptr },
} as const;

type Library = ReturnType<typeof dlopen<typeof symbols>>;
export type Operation = Exclude<keyof typeof symbols, "prelude_abi_version" | "prelude_free">;

let chosenPath: string | undefined;
let library: Library | undefined;

const platform = `${process.platform}-${process.arch}`;

/**
 * This machine's libprelude from the package's lib/<os>-<cpu>/, which ships
 * one for each supported platform (see scripts/npm-package.ts).
 */
export function platformLibrary(root = new URL("../../", import.meta.url)): string | undefined {
  const path = fileURLToPath(new URL(`lib/${platform}/libprelude.${suffix}`, root));
  return existsSync(path) ? path : undefined;
}

/**
 * Uses a specific libprelude instead of PRELUDE_LIB or the bundled one, such
 * as one embedded in a single-file executable:
 *
 *     import lib from "./libprelude.so" with { type: "file" }
 *     Library.use(lib)
 */
export function use(path: string): void {
  if (library !== undefined && path !== chosenPath) {
    throw new Error("prelude: libprelude is already loaded; call Library.use() before the first menu, motd, or docs call");
  }
  chosenPath = path;
}

function load(): Library {
  if (library !== undefined) return library;
  const path = chosenPath ?? (process.env.PRELUDE_LIB || platformLibrary());
  if (!path) {
    throw new Error(
      `prelude: no libprelude for ${platform}. The package ships one for macOS and Linux on arm64 and x64;` +
        " in a checkout, build it with `bun scripts/npm-package.ts library`. Otherwise set PRELUDE_LIB or call Library.use().",
    );
  }
  const opened = dlopen(path, symbols);
  const version = opened.symbols.prelude_abi_version();
  if (version !== abiVersion) {
    opened.close();
    throw new Error(`prelude: ${path} speaks libprelude ABI ${version}, but this package needs ${abiVersion}`);
  }
  chosenPath = path;
  library = opened;
  return opened;
}

/**
 * Runs one libprelude operation. Interactive operations (the picker, the docs
 * viewer) take over the terminal and block until the person leaves them.
 */
export function call<Result>(operation: Operation, request: unknown): Result {
  const { symbols } = load();
  const input = Buffer.from(`${JSON.stringify(request)}\0`);
  const reply = symbols[operation](ptr(input));
  if (reply === null) throw new Error(`prelude: ${operation} returned no reply`);
  try {
    const { ok, error } = JSON.parse(new CString(reply).toString()) as { ok?: Result; error?: string };
    if (error !== undefined) throw new Error(error);
    return ok as Result;
  } finally {
    symbols.prelude_free(reply);
  }
}
