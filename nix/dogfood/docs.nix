# Showcase: hand-author the project manual independently from menu tasks.
{ ... }:
{
  prelude.docs = {
    enable = true;
    sections = {
      name = {
        order = 100;
        blocks = [
          {
            type = "lead";
            term = "prelude";
            text = "devshell UI suite — MOTD, command menu, and this manual";
          }
        ];
      };

      synopsis = {
        order = 200;
        blocks = [
          {
            type = "shell";
            command = "help";
            note = "or ? — reprint the welcome banner";
          }
          {
            type = "shell";
            command = "menu";
            note = "or m — interactive command picker";
          }
          {
            type = "shell";
            command = "docs";
            note = "or d — this manual (digits jump sections)";
          }
          {
            type = "shell";
            command = "menu list";
            note = "print the task table without a TTY";
          }
        ];
      };

      description = {
        order = 300;
        blocks = [
          {
            type = "para";
            text = "prelude is a flake-parts module suite for devshell UI. Nix validates and normalizes configuration at build time, then embeds JSON into small Go binaries: Lip Gloss for the static MOTD and Bubble Tea for the interactive menu and this docs viewer.";
          }
          {
            type = "para";
            text = "On shell entry the MOTD prints a header, description, next-step commands, and optional recipes. The menu fuzzy-filters the tasks declared under prelude.groups. Docs content is written by hand under prelude.docs.sections — it is never invented from the task catalog.";
          }
        ];
      };

      options = {
        order = 400;
        blocks = [
          {
            type = "option";
            term = "help, ?";
            text = "Reprint the MOTD welcome banner.";
          }
          {
            type = "option";
            term = "menu, m";
            text = "Open the interactive command picker. Pass a task name or key to run it directly.";
          }
          {
            type = "option";
            term = "docs, d";
            text = "Open this manual. Digits 1–N jump to sections; j/k scroll; q quits.";
          }
          {
            type = "option";
            term = "prelude.theme";
            text = "Named palette (phosphor, prelude, nord, …). Override individual tokens with prelude.palette.";
          }
          {
            type = "option";
            term = "prelude.colorProfile";
            text = "auto | truecolor | ansi256. Force truecolor when the terminal mis-detects depth.";
          }
        ];
      };

      commands = {
        order = 500;
        blocks = [
          {
            type = "command";
            term = "motd";
            text = "Print the static welcome banner (also: help, ?).";
          }
          {
            type = "command";
            term = "menu";
            text = "Interactive picker over prelude.groups tasks. Tab expands details; enter runs.";
          }
          {
            type = "command";
            term = "menu list";
            text = "Print the grouped task table and exit (non-interactive).";
          }
          {
            type = "command";
            term = "docs";
            text = "This manual. Content comes only from prelude.docs.sections.";
          }
          {
            type = "command";
            term = "nix flake check";
            text = "Build packages and run render smoke tests for this flake.";
          }
        ];
      };

      examples = {
        order = 600;
        blocks = [
          {
            type = "example";
            command = "nix develop";
            note = "enter the shell; MOTD prints automatically";
          }
          {
            type = "example";
            command = "docs";
            note = "open this manual; press 5 to jump to COMMANDS";
          }
          {
            type = "example";
            command = "menu check";
            note = "run the check task by name";
          }
          {
            type = "example";
            command = "nix run .#example-motd";
            note = "acme-web MOTD demo";
          }
        ];
      };

      see-also = {
        order = 700;
        title = "see also";
        blocks = [
          {
            type = "para";
            text = "README.md — module options, themes, and downstream usage.";
          }
          {
            type = "para";
            text = "nix run .#examples — tour every feature demo.";
          }
          {
            type = "para";
            text = "src/prelude/options/ — option declarations for motd, menu, and docs.";
          }
        ];
      };
    };
  };
}
