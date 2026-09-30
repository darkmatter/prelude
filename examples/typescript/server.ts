// A normal module: `serve` is its API, used by the rest of the app.
// `preludeCommand` wraps that function for the menu. It has no name of its
// own; main.ts decides what people type to run it.
import { Command } from "@drkmttr/prelude";

export function serve(port: number, host = "127.0.0.1") {
  const server = Bun.serve({ port, hostname: host, fetch: () => new Response("hello from acme\n") });
  console.log(`listening on ${server.url}`);
}

export const preludeCommand = Command.make({
  description: "run the dev server",
  args: [
    { token: "--port", description: "port to listen on", type: "number", default: 3000 },
    { token: "--host", description: "interface to bind", options: ["127.0.0.1", "0.0.0.0"] },
  ],
  // Typed from the declarations above:
  // args.port is a number, args.host is "127.0.0.1" | "0.0.0.0" | undefined.
  run: (args) => serve(args.port, args.host),
  motd: 1, // first under Getting Started on the welcome banner
});

export default serve;
