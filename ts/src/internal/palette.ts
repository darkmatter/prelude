import type * as Palette from "../Palette.ts";
import { defaults } from "./generated.ts";

const colorProfiles: readonly Palette.ColorProfile[] = ["auto", "truecolor", "ansi256"];

export function resolveColorProfile(profile: Palette.ColorProfile = defaults.colorProfile): Palette.ColorProfile {
  if (!colorProfiles.includes(profile)) {
    throw new Error(`prelude: colorProfile must be one of ${colorProfiles.join(", ")}, got "${profile}"`);
  }
  return profile;
}

/**
 * Paints text for a stream: plain when it is not a terminal or NO_COLOR is
 * set, otherwise in the palette color at the terminal's depth.
 */
export function paint(color: Palette.Color, text: string, stream: { isTTY?: boolean } = process.stdout): string {
  if (!stream.isTTY || process.env.NO_COLOR) return text;
  const start = typeof color === "number" ? `\x1b[38;5;${color}m` : Bun.color(color, "ansi");
  return start ? `${start}${text}\x1b[39m` : text;
}
