// The entry point for a single-file executable. It embeds the Go library, so
// the executable runs without PRELUDE_LIB, then starts the app:
//
//   cp "$PRELUDE_LIB" examples/typescript/libprelude.so
//   bun build --compile examples/typescript/standalone.ts --outfile acme
import { Library } from "@drkmttr/prelude";

import library from "./libprelude.so" with { type: "file" };

Library.use(library);
await import("./main.ts");
