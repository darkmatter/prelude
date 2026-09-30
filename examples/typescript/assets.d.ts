// Types for the non-code imports: Markdown arrives as text, the library as a
// file path (inside the executable after `bun build --compile`).
declare module "*.md" {
  const text: string;
  export default text;
}

declare module "*.so" {
  const path: string;
  export default path;
}
