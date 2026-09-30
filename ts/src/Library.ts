/**
 * libprelude, the Go shared library the menu, MOTD, and docs run in. It loads
 * the path given to `use()`, else PRELUDE_LIB, else the one the package ships
 * for this machine under lib/<os>-<cpu>/.
 */
export { use } from "./internal/ffi.ts";
