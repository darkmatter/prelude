import { defaults, themes } from "./internal/generated.ts";

/** Prelude's themes (src/prelude/themes.nix), keyed by name. */
export { themes };

export type Theme = keyof typeof themes;
export type Token = keyof (typeof themes)["minted"];
/** A hex color (`#rrggbb`) or an ANSI-256 index. */
export type Color = string | number;
export type Palette = Record<Token, Color>;
/** `auto` detects the terminal; the others force a color depth. */
export type ColorProfile = "auto" | "truecolor" | "ansi256";

/** The look options every surface accepts. */
export interface Options {
  /** A theme from src/prelude/themes.nix. Default: `minted`. */
  theme?: Theme;
  /** Per-token overrides applied over the theme. */
  palette?: Partial<Palette>;
  /** Color depth. Default: `truecolor`. */
  colorProfile?: ColorProfile;
}

/** Mirrors `resolvePalette` in src/prelude/lib.nix: theme tokens, then overrides. */
export function resolve(theme: Theme = defaults.theme, overrides: Partial<Palette> = {}): Palette {
  const base = themes[theme];
  if (base === undefined) {
    throw new Error(`prelude: unknown theme "${theme}" (expected one of: ${Object.keys(themes).join(", ")})`);
  }
  const palette: Palette = { ...base };
  for (const [token, value] of Object.entries(overrides)) {
    if (value != null) palette[token as Token] = value;
  }
  return palette;
}
