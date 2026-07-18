# Showcase: each Markdown file becomes one page in the docs viewer.
{ ... }:
{
  prelude.docs = {
    pages = [
      { text = ./docs/welcome.md; }
      { text = ./docs/this-shell.md; }
      { text = ./docs/commands.md; }
      { text = ./docs/your-own-repo.md; }
      { text = ./docs/configuration.md; }
      { text = ./docs/see-also.md; }
    ];
  };
}
