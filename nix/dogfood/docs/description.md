# Description

Prelude is a flake-parts module suite for devshell UI. Nix validates and normalizes configuration at build time, then embeds JSON into small Go binaries built with Lip Gloss and Bubble Tea.

On shell entry the MOTD introduces the project and its next steps. The menu fuzzy-filters commands declared under `prelude.commands`. This docs viewer reads ordinary Markdown files declared under `prelude.docs.pages`.

Because pages are Markdown, prose can use **emphasis**, `inline code`, lists, links, block quotes, and fenced code without translating everything into Nix block objects.
