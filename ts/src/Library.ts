/**
 * libprelude, the Go shared library the menu, MOTD, and docs run in. It loads
 * from PRELUDE_LIB unless `use()` names another path first.
 */
export { use } from "./internal/ffi.ts";
