/**
 * @drkmttr/prelude: prelude's command menu, MOTD, and docs viewer for Bun
 * apps. Commands are TypeScript functions (or shell text) declared next to
 * the code they wrap; the Go surfaces run in-process through libprelude.
 * Every module is a namespace with a `make` constructor:
 *
 *     import { Command, Prelude } from "@drkmttr/prelude"
 *
 *     const dev = Command.make({ run: () => serve() })
 *     await Prelude.make({ project: "acme", commands: { dev } }).main()
 */
export * as Args from "./Args.ts";
export * as Command from "./Command.ts";
export * as Docs from "./Docs.ts";
export * as Library from "./Library.ts";
export * as Menu from "./Menu.ts";
export * as Motd from "./Motd.ts";
export * as Palette from "./Palette.ts";
export * as Prelude from "./Prelude.ts";
