# Showcase: each Markdown file becomes one page in the docs viewer.
{ ... }:
{
  prelude.docs = {
    pages = [
      { text = ./docs/name.md; }
      { text = ./docs/synopsis.md; }
      { text = ./docs/description.md; }
      { text = ./docs/options.md; }
      { text = ./docs/commands.md; }
      { text = ./docs/examples.md; }
      { text = ./docs/see-also.md; }
    ];
  };
}
