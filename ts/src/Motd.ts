import { basename } from "node:path";

import type * as Command from "./Command.ts";
import * as Palette from "./Palette.ts";
import { catalogue, compare, gettingStarted } from "./internal/catalogue.ts";
import { call } from "./internal/ffi.ts";
import { defaults } from "./internal/generated.ts";
import { resolveColorProfile } from "./internal/palette.ts";
import { defaultDispatcher } from "./internal/process.ts";

// A port of src/prelude/motd.nix: options in, the Go renderer's normalized
// Config out. The conformance test compares it with Nix's output for shared
// fixtures. Unlike Nix, the defaults carry no ACME example content.

export type Align = "left" | "center" | "right";
export type TitleStyle = "plain" | "spine" | "bracketed" | "label" | "inline" | "inverted";
/** false/null: transparent; true: the theme's bg; a color; or a shade/blend relative to the terminal. */
export type Background = boolean | null | Palette.Color | { relative: number } | { blend: number };
export type HeaderBackground = boolean | null | Palette.Color | { relative: number };

/** Sides in cells; `x`/`y` are axis shorthands that explicit sides override. */
export interface Spacing {
  x?: number;
  y?: number;
  top?: number | null;
  bottom?: number | null;
  left?: number | null;
  right?: number | null;
  /** Vertical sides apply only on terminals at least this tall; 0 always. */
  minHeight?: number;
}

export interface Link {
  label: string;
  url: string;
}

/** A check result: ok or failed, a first line of output meaning ok, or both explicitly. */
export type CheckOutcome = boolean | string | { ok: boolean; output?: string };

/** A header badge: static text, a shell check, or a TypeScript check. */
export interface StatusItem {
  label?: string;
  /** Static text for a badge without a check. */
  status?: string;
  /** Shell text run with bash (ok on exit 0), or a function that decides. */
  check?: string | (() => CheckOutcome | Promise<CheckOutcome>);
  /** Text when the check passes without output. */
  ok?: string;
  /** Text when the check fails without output. */
  fail?: string;
  failLevel?: "error" | "warning";
  /** `light` shows only the colored dot; `diagnostic` adds output below. */
  output?: "" | "light" | "diagnostic";
  /** Badge order (ascending, then key). Default: 1000. */
  order?: number;
}

/** An env chip: a fixed value, or a shell probe / function whose result is shown. */
export type EnvItem =
  | { label: string; value: string }
  | { label: string; probe: string | (() => string | Promise<string>) };

export interface RecipeStep {
  command?: string;
  comment?: string;
}

/** A titled multi-step workflow. `lines` is the legacy form: `# …` lines become comments. */
export interface Recipe {
  title?: string;
  order?: number;
  steps?: readonly RecipeStep[];
  lines?: readonly string[];
}

export interface StyledText {
  text?: string;
  foreground?: Palette.Color | null;
  background?: Background;
  bold?: boolean;
  italic?: boolean;
  faint?: boolean;
  /** Follow-on lines; wrap commands in backticks for accent. */
  tips?: readonly string[];
}

/** `prelude.motd.*`, plus the commands whose `motd` position lists them on Getting Started. */
export interface Options extends Palette.Options {
  project?: string;
  /** Multiline title text (e.g. from `prelude title`); null draws the styled project name. */
  title?: { text?: string | null; align?: Align; style?: TitleStyle };
  background?: Background;
  border?: boolean;
  clearScreen?: boolean;
  margin?: Spacing;
  padding?: Spacing;
  align?: Align;
  verticalAlign?: "top" | "center" | "bottom";
  header?: {
    tagline?: { text?: string; subtitle?: string | null; layout?: "stack" | "inline"; align?: "left" | "center" };
    background?: HeaderBackground;
    statusHint?: { layout?: "below" | "inline"; links?: readonly Link[] };
    status?: Readonly<Record<string, StatusItem>>;
  };
  description?: StyledText;
  links?: readonly Link[];
  env?: readonly EnvItem[];
  recipes?: Readonly<Record<string, Recipe>>;
  gettingStarted?: { heading?: string; commandsLabel?: string; examplesLabel?: string; commandNote?: string };
  shortcuts?: readonly { command: string; alias?: string }[];
  width?: number | "full";
  maxWidth?: number | null;
  commands?: Readonly<Record<string, Command.Any>>;
  groupOrder?: readonly string[];
  /** How Getting Started shows commands being run (`acme dev`). Default: derived from process.argv. */
  dispatcher?: string;
}

type ResolvedSpacing = { top: number; bottom: number; left: number; right: number; minHeight: number };
type HeaderStatus = {
  label: string;
  status: string;
  check: string;
  async: boolean;
  ok: string;
  fail: string;
  failLevel: string;
  output: string;
};

/** The MOTD's JSON boundary, internal/motd.Config in Go. */
export interface Config {
  project: string;
  title: string;
  titleAlign: Align;
  colorProfile: Palette.ColorProfile;
  palette: Palette.Palette;
  background: string;
  backgroundRelative: number;
  backgroundBlend: number;
  backgroundBlendSet: boolean;
  border: boolean;
  clearScreen: boolean;
  margin: ResolvedSpacing;
  align: Align;
  verticalAlign: "top" | "center" | "bottom";
  padding: ResolvedSpacing;
  header: {
    titleStyle: TitleStyle;
    tagline: string;
    subtitle: string | null;
    taglineLayout: string;
    taglineAlign: string;
    statusHintLayout: string;
    statusHintLinks: Link[];
    status: HeaderStatus[];
    background: string;
    backgroundRelative: number;
    backgroundRaised: boolean;
  };
  description: {
    text: string;
    foreground: string;
    background: string;
    backgroundRelative: number;
    bold: boolean;
    italic: boolean;
    faint: boolean;
    tips: string[];
  };
  links: Link[];
  env: { label: string; value: string; probe: string }[];
  commands: { command: string; description: string }[];
  recipes: { title: string; steps: { command: string; comment: string }[] }[];
  gettingStarted: { heading: string; commandsLabel: string; examplesLabel: string; commandNote?: string };
  shortcuts: { command: string; alias: string }[];
  width: number;
  maxWidth: number;
}

// Checks and probes written as functions run here; Go sees them by these ids.
const statusCheckId = (key: string) => `ts:status:${key}`;
const envProbeId = (index: number) => `ts:env:${index}`;

/** Object spread that, like Nix's `//` over optional fields, skips unset (undefined) values. */
function merge<T extends object>(base: T, over: object | undefined): T {
  const merged = { ...base } as Record<string, unknown>;
  for (const [key, value] of Object.entries(over ?? {})) {
    if (value !== undefined) merged[key] = value;
  }
  return merged as T;
}

function oneOf<T extends string>(what: string, value: T, allowed: readonly T[]): T {
  if (!allowed.includes(value)) throw new Error(`prelude: motd ${what} must be one of ${allowed.join(", ")}, got "${value}"`);
  return value;
}

const jsonColor = (value: Palette.Color | null | undefined): string => (value == null ? "" : String(value));

/** Mirrors `sortOrderedAttrs`: by `order` (default 1000), then key. */
function sortOrdered<T extends { order?: number }>(attrs: Readonly<Record<string, T>>): [string, T][] {
  return Object.entries(attrs).sort(
    ([aKey, a], [bKey, b]) => (a.order ?? 1000) - (b.order ?? 1000) || compare(aKey, bKey),
  );
}

/** Mirrors `resolveSpacing`: explicit sides win over the x/y axis shorthands. */
function resolveSpacing(spacing: Spacing): ResolvedSpacing {
  return {
    top: spacing.top ?? spacing.y ?? 0,
    bottom: spacing.bottom ?? spacing.y ?? 0,
    left: spacing.left ?? spacing.x ?? 0,
    right: spacing.right ?? spacing.x ?? 0,
    minHeight: spacing.minHeight ?? 0,
  };
}

function splitBackground(value: Background | undefined, palette: Palette.Palette) {
  if (value == null || value === false) return { color: null, relative: 0, blend: 0, blendSet: false };
  if (value === true) return { color: palette.bg, relative: 0, blend: 0, blendSet: false };
  if (typeof value === "object" && "relative" in value) return { color: null, relative: value.relative, blend: 0, blendSet: false };
  if (typeof value === "object") return { color: null, relative: 0, blend: value.blend, blendSet: true };
  return { color: value, relative: 0, blend: 0, blendSet: false };
}

function splitHeaderBackground(value: HeaderBackground | undefined) {
  if (value == null || value === false) return { color: null, relative: 0, raised: false };
  if (value === true) return { color: null, relative: 0, raised: true };
  if (typeof value === "object") return { color: null, relative: value.relative, raised: false };
  return { color: value, relative: 0, raised: false };
}

/** Mirrors `lineToStep`: drops blank lines; `#`/`# ` lines become comments. */
function lineToStep(line: string): { command: string; comment: string } | null {
  const trimmed = line.replace(/ $/, "").replace(/^ /, "");
  if (trimmed === "") return null;
  if (trimmed.startsWith("#")) return { command: "", comment: trimmed.slice(trimmed.startsWith("# ") ? 2 : 1) };
  return { command: trimmed, comment: "" };
}

/** Builds the MOTD config the way src/prelude/motd.nix does. */
function buildConfig(options: Options): Config {
  const d = defaults.motd;
  const palette = Palette.resolve(options.theme, options.palette);
  const dispatcher = options.dispatcher ?? defaultDispatcher();

  const title = merge(d.title, options.title);
  const header = merge(d.header, options.header);
  const tagline = merge(d.header.tagline, options.header?.tagline);
  const statusHint = merge(d.header.statusHint, options.header?.statusHint);
  const headerBackground = splitHeaderBackground(header.background);
  const card = splitBackground(options.background ?? d.background, palette);

  const description = merge(
    merge({ text: "", foreground: null, background: null, bold: false, italic: false, faint: false }, d.description),
    options.description,
  ) as StyledText;
  const descriptionBackground = splitBackground(description.background, palette);

  const status = sortOrdered(header.status as Readonly<Record<string, StatusItem>>)
    .map(([key, item]) => ({
      label: item.label ?? "",
      status: item.status ?? "",
      check: typeof item.check === "function" ? statusCheckId(key) : (item.check ?? ""),
      // Checks run before every render here; async would advertise the
      // devshell's `r` reload, which a TypeScript app does not have.
      async: false,
      ok: item.ok ?? "ok",
      fail: item.fail ?? "fail",
      failLevel: item.failLevel ?? "error",
      output: item.output ?? "",
    }))
    .filter((item) => item.label !== "" || item.status !== "" || item.check !== "");

  const env = (options.env ?? d.env).map((item: EnvItem, index) => {
    const value = "value" in item ? item.value : undefined;
    const probe = "probe" in item ? item.probe : undefined;
    if ((value == null) === (probe == null)) {
      throw new Error(`prelude: motd env item "${item.label}" must set exactly one of \`value\` or \`probe\``);
    }
    return {
      label: item.label,
      value: value ?? "",
      probe: typeof probe === "function" ? envProbeId(index) : (probe ?? ""),
    };
  });

  const recipes = sortOrdered(options.recipes ?? {}).map(([name, recipe]) => ({
    title: recipe.title ?? name,
    steps:
      recipe.steps !== undefined && recipe.steps.length > 0
        ? recipe.steps.map((step) => ({ command: step.command ?? "", comment: step.comment ?? "" }))
        : (recipe.lines ?? []).map(lineToStep).filter((step) => step !== null),
  }));

  const width = options.width ?? d.width;
  if (width !== "full" && !Number.isInteger(width)) throw new Error('prelude: motd width must be an integer or "full"');
  const maxWidth = options.maxWidth === undefined ? d.maxWidth : options.maxWidth;
  if (maxWidth !== null && !Number.isInteger(maxWidth)) throw new Error("prelude: motd maxWidth must be an integer or null");

  return {
    project: options.project ?? basename(process.cwd()),
    title: title.text ?? "",
    titleAlign: oneOf("title.align", title.align, ["left", "center", "right"]),
    colorProfile: resolveColorProfile(options.colorProfile),
    palette,
    background: jsonColor(card.color),
    backgroundRelative: card.relative,
    backgroundBlend: card.blend,
    backgroundBlendSet: card.blendSet,
    border: options.border ?? d.border,
    clearScreen: options.clearScreen ?? d.clearScreen,
    margin: resolveSpacing(merge(d.margin, options.margin)),
    align: oneOf("align", options.align ?? d.align, ["left", "center", "right"]),
    verticalAlign: oneOf("verticalAlign", options.verticalAlign ?? d.verticalAlign, ["top", "center", "bottom"]),
    padding: resolveSpacing(merge(d.padding, options.padding)),
    header: {
      titleStyle: oneOf("title.style", title.style, ["plain", "spine", "bracketed", "label", "inline", "inverted"]),
      tagline: tagline.text,
      subtitle: (tagline as { subtitle?: string | null }).subtitle ?? null,
      taglineLayout: oneOf("header.tagline.layout", tagline.layout, ["stack", "inline"]),
      taglineAlign: oneOf("header.tagline.align", tagline.align, ["left", "center"]),
      statusHintLayout: oneOf("header.statusHint.layout", statusHint.layout, ["below", "inline"]),
      statusHintLinks: [...statusHint.links],
      status,
      background: jsonColor(headerBackground.color),
      backgroundRelative: headerBackground.relative,
      backgroundRaised: headerBackground.raised,
    },
    description: {
      text: description.text ?? "",
      bold: description.bold ?? false,
      italic: description.italic ?? false,
      faint: description.faint ?? false,
      backgroundRelative: descriptionBackground.relative,
      tips: [...(description.tips ?? [])],
      foreground: jsonColor(description.foreground ?? palette.fg),
      // A concrete description fill wins; a relative one resolves at render
      // time; otherwise the card's concrete fill carries through.
      background: jsonColor(
        descriptionBackground.color ?? (descriptionBackground.relative !== 0 ? null : card.color),
      ),
    },
    links: [...(options.links ?? d.links)],
    env,
    commands: gettingStarted(catalogue(options.commands ?? {}, options.groupOrder)).map((entry) => ({
      command: `${dispatcher} ${entry.key}`,
      description: entry.command.description ?? "",
    })),
    recipes,
    gettingStarted: merge(d.gettingStarted, options.gettingStarted),
    shortcuts: (options.shortcuts ?? []).map((shortcut) => ({ command: shortcut.command, alias: shortcut.alias ?? "" })),
    width: width === "full" ? 0 : width,
    maxWidth: maxWidth ?? 0,
  };
}

function outcome(result: CheckOutcome): { ok: boolean; output: string } {
  if (typeof result === "boolean") return { ok: result, output: "" };
  if (typeof result === "string") return { ok: true, output: result };
  return { ok: result.ok, output: result.output ?? "" };
}

/** Runs the checks and probes written as functions, as Preflight would run shell ones. */
async function hostedResults(options: Options) {
  const checks = Object.entries(options.header?.status ?? {}).flatMap(([key, item]) =>
    typeof item.check === "function" ? [{ id: statusCheckId(key), check: item.check }] : [],
  );
  const probes = (options.env ?? []).flatMap((item, index) =>
    "probe" in item && typeof item.probe === "function" ? [{ id: envProbeId(index), probe: item.probe }] : [],
  );
  const [status, env] = await Promise.all([
    Promise.all(
      checks.map(async ({ id, check }) => {
        try {
          return { check: id, ...outcome(await check()) };
        } catch (error) {
          return { check: id, ok: false, output: error instanceof Error ? error.message : String(error) };
        }
      }),
    ),
    Promise.all(
      probes.map(async ({ id, probe }) => {
        try {
          return { probe: id, value: String(await probe()) };
        } catch {
          // A failing probe hides its chip, like a failing shell probe.
          return { probe: id, value: "" };
        }
      }),
    ),
  ]);
  return { status, env };
}

/** The terminal size to lay out for; 0 lets the renderer fall back to 80×24. */
export interface RenderSize {
  width?: number;
  height?: number;
}

export interface Motd {
  /** The config handed to the Go renderer. */
  readonly config: Config;
  /**
   * Renders the banner: function checks and probes run here, shell ones in
   * Go, then the same renderer as the `motd` command lays it out.
   */
  render(size?: RenderSize): Promise<string>;
  /** Renders the banner to stdout. */
  print(): Promise<void>;
}

/** A welcome banner; options mirror `prelude.motd.*`. */
export function make(options: Options = {}): Motd {
  const config = buildConfig(options);
  const render = async (size: RenderSize = {}): Promise<string> =>
    call<string>("prelude_motd_render", {
      config,
      results: await hostedResults(options),
      width: size.width ?? process.stdout.columns ?? 0,
      height: size.height ?? process.stdout.rows ?? 0,
    });
  return {
    config,
    render,
    print: async () => {
      process.stdout.write(await render());
    },
  };
}
