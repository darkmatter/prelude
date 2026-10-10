# The plain modules every Prelude evaluation starts from: the option
# declarations and Prelude's own catalogue entries. flakeModules.default
# imports them; lib.evalModule evaluates them directly.
[
  ./options/shared.nix
  ./options/motd.nix
  ./options/menu.nix
  ./options/portal.nix
  ./options/docs.nix
  ./options/prompt.nix
  ./options/workspace.nix
  ./builtin-commands.nix
]
