import { argName } from "./internal/args.ts";

/**
 * One declared argument, in the shape of `prelude.commands.<key>.args` in Nix:
 * an option (`--port`), a flag (`--open` with `boolean`), or a positional
 * (`<target>`). A command's `run` receives each value under its camelCase name.
 */
export interface Spec {
  /** `--name` for an option or flag; `<name>` (or `NAME`) for a positional. */
  readonly token: string;
  readonly description?: string;
  /** Fail when the argument is missing and has no default. */
  readonly required?: boolean;
  /** A flag that takes no value: true when present, else false. */
  readonly boolean?: boolean;
  /** Parse the value as a number. Default: string. */
  readonly type?: "string" | "number";
  /** The allowed values, also offered as chips in the picker. */
  readonly options?: readonly string[];
  /** The value when the argument is omitted. */
  readonly default?: string | number;
}

type Stripped<Token extends string> = Token extends `--${infer Name}`
  ? Name
  : Token extends `<${infer Name}>`
    ? Name
    : Token;
type Camel<Name extends string> = Name extends `${infer Head}-${infer Rest}` ? `${Head}${Capitalize<Camel<Rest>>}` : Name;

/** The property a token's value is stored under: `--dry-run` → `dryRun`, `<target>` → `target`. */
export type Name<Token extends string> = Camel<Stripped<Token>>;

type ValueOf<S extends Spec> = S extends { boolean: true }
  ? boolean
  : S extends { type: "number" }
    ? number
    : S extends { options: readonly (infer Option extends string)[] }
      ? Option
      : string;
type Present<S extends Spec> = S extends { required: true } | { default: string | number } | { boolean: true } ? S : never;
type Simplify<T> = { [Key in keyof T]: T[Key] } & {};

/** The parsed values `run` receives for a declared argument list. */
export type Values<Specs extends readonly Spec[]> = Simplify<
  { [S in Present<Specs[number]> as Name<S["token"]>]: ValueOf<S> } & {
    [S in Exclude<Specs[number], Present<Specs[number]>> as Name<S["token"]>]?: ValueOf<S>;
  }
>;

/** Invalid argument words: the message is meant for the person who typed them. */
export class ParseError extends Error {
  override name = "ParseError";
}

/**
 * Parses argument words against their declarations: `--name value`,
 * `--name=value`, bare `--flag`, positionals in declaration order, and `--`
 * to end option parsing. Applies defaults and types; throws ParseError.
 */
export function parse<const Specs extends readonly Spec[]>(specs: Specs, argv: readonly string[]): Values<Specs> {
  const options = new Map<string, Spec>();
  const positionals: Spec[] = [];
  for (const spec of specs) {
    if (spec.token.startsWith("--")) options.set(spec.token, spec);
    else positionals.push(spec);
  }

  const given = new Map<Spec, string | true>();
  let position = 0;
  let literal = false;
  for (let index = 0; index < argv.length; index++) {
    const word = argv[index]!;
    if (!literal && word === "--") {
      literal = true;
      continue;
    }
    if (!literal && word.startsWith("--")) {
      const equals = word.indexOf("=");
      const token = equals < 0 ? word : word.slice(0, equals);
      const spec = options.get(token);
      if (spec === undefined) throw new ParseError(`unknown option ${token}`);
      if (spec.boolean) {
        if (equals >= 0) throw new ParseError(`${token} takes no value`);
        given.set(spec, true);
        continue;
      }
      const value = equals >= 0 ? word.slice(equals + 1) : argv[++index];
      if (value === undefined) throw new ParseError(`${token} needs a value`);
      given.set(spec, value);
      continue;
    }
    const spec = positionals[position++];
    if (spec === undefined) throw new ParseError(`unexpected argument "${word}"`);
    given.set(spec, word);
  }

  const values: Record<string, unknown> = {};
  for (const spec of specs) {
    const name = argName(spec.token);
    const value = given.get(spec);
    if (spec.boolean) {
      values[name] = value === true;
    } else if (value !== undefined) {
      values[name] = coerce(spec, value as string);
    } else if (spec.default !== undefined) {
      values[name] = typeof spec.default === "string" ? coerce(spec, spec.default) : spec.default;
    } else if (spec.required) {
      throw new ParseError(`missing required argument ${spec.token}`);
    }
  }
  return values as Values<Specs>;
}

function coerce(spec: Spec, value: string): string | number {
  if (spec.options && !spec.options.includes(value)) {
    throw new ParseError(`${spec.token} must be one of ${spec.options.join(", ")}; got "${value}"`);
  }
  if (spec.type !== "number") return value;
  const number = Number(value);
  if (value.trim() === "" || Number.isNaN(number)) {
    throw new ParseError(`${spec.token} expects a number; got "${value}"`);
  }
  return number;
}

/**
 * Splits argument text typed into the picker into words the way a POSIX shell
 * would, without expansion: whitespace separates, quotes group, and a
 * backslash escapes the next character (inside double quotes only `"`, `\`,
 * `$`, and a backtick).
 */
export function split(line: string): string[] {
  const words: string[] = [];
  let word = "";
  let inWord = false;
  let quote: "'" | '"' | null = null;
  for (let index = 0; index < line.length; index++) {
    const char = line[index]!;
    if (quote === "'") {
      if (char === "'") quote = null;
      else word += char;
    } else if (quote === '"') {
      const next = line[index + 1];
      if (char === '"') quote = null;
      else if (char === "\\" && next !== undefined && '"\\$`'.includes(next)) word += line[++index];
      else word += char;
    } else if (char === "'" || char === '"') {
      quote = char;
      inWord = true;
    } else if (char === "\\") {
      if (index + 1 < line.length) word += line[++index];
      inWord = true;
    } else if (/\s/.test(char)) {
      if (inWord) words.push(word);
      word = "";
      inWord = false;
    } else {
      word += char;
      inWord = true;
    }
  }
  if (quote !== null) throw new ParseError(`unterminated ${quote} quote`);
  if (inWord) words.push(word);
  return words;
}
