import { basename } from "node:path";

import * as Palette from "./Palette.ts";
import { call } from "./internal/ffi.ts";
import { resolveColorProfile } from "./internal/palette.ts";

/** A docs page, or a sidebar group when it has children. */
export interface Page {
  /** Sidebar title. Default for a page: its first `# heading`. */
  title?: string;
  /**
   * The page body. Bundle files as text so single-file executables carry
   * them: `import guide from "./guide.md" with { type: "text" }`.
   */
  markdown?: string;
  children?: readonly Page[];
  /** A blank sidebar row above this entry. */
  gapBefore?: boolean;
}

export interface Options extends Palette.Options {
  project?: string;
  pages: readonly Page[];
}

type NavNode = {
  kind: "leaf" | "group";
  title: string;
  markdown?: string;
  children?: NavNode[];
  gapBefore?: boolean;
};

/** The docs viewer's JSON boundary, internal/docs.Config in Go. */
export interface Config {
  project: string;
  colorProfile: Palette.ColorProfile;
  palette: Palette.Palette;
  nav: NavNode[];
}

export interface Docs {
  /** The config handed to the Go viewer. */
  readonly config: Config;
  /** Renders one page (1-based, depth first) as `docs <page>` prints it. */
  render(at?: { page?: number; width?: number }): string;
  /** Opens the full-screen docs viewer; returns when the person quits it. */
  open(): void;
  /** The number of pages: valid `page` values run 1..pages(). */
  pages(): number;
}

function navNode(page: Page): NavNode {
  const gap = page.gapBefore ? { gapBefore: true } : {};
  if (page.children !== undefined && page.children.length > 0) {
    return { kind: "group", title: page.title ?? "", children: page.children.map(navNode), ...gap };
  }
  if (!page.markdown) {
    throw new Error(`prelude: docs page "${page.title ?? "(untitled)"}" needs markdown or children`);
  }
  return { kind: "leaf", title: page.title ?? "", markdown: page.markdown, ...gap };
}

function countPages(pages: readonly Page[]): number {
  return pages.reduce((total, page) => total + (page.children?.length ? countPages(page.children) : 1), 0);
}

/** A docs viewer over Markdown pages. */
export function make(options: Options): Docs {
  if (options.pages.length === 0) throw new Error("prelude: docs needs at least one page");
  const config: Config = {
    project: options.project ?? basename(process.cwd()),
    colorProfile: resolveColorProfile(options.colorProfile),
    palette: Palette.resolve(options.theme, options.palette),
    nav: options.pages.map(navNode),
  };
  return {
    config,
    render: ({ page = 1, width } = {}) =>
      call<{ output: string; pages: number }>("prelude_docs_render", {
        config,
        page,
        width: width ?? process.stdout.columns ?? 80,
      }).output,
    open: () => {
      call("prelude_docs_open", { config });
    },
    pages: () => countPages(options.pages),
  };
}
