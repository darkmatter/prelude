package menu

import (
	"strings"

	"prelude/pkg/manual"
)

// helpDocument adapts task configuration into the shared manual presentation model.
func helpDocument(cfg *Config) manual.Document {
	project := cfg.Project
	sections := []manual.Section{
		{
			Title: "name",
			Blocks: []manual.Block{richBlock(4, false,
				manual.Span{Role: manual.Accent2, Text: project, Bold: true},
				manual.Span{Role: manual.Foreground, Text: " — devshell UI: welcome banner, command menu, and docs"},
			)},
		},
		{
			Title: "synopsis",
			Blocks: []manual.Block{
				richBlock(4, false,
					manual.Span{Role: manual.Accent2, Text: "help", Bold: true},
					manual.Span{Role: manual.Muted, Text: " | "},
					manual.Span{Role: manual.Accent2, Text: "?", Bold: true},
					manual.Span{Role: manual.Muted, Text: "              reprint the welcome banner (motd)"},
				),
				richBlock(4, false,
					manual.Span{Role: manual.Accent2, Text: "menu", Bold: true},
					manual.Span{Role: manual.Muted, Text: " | "},
					manual.Span{Role: manual.Accent2, Text: "m", Bold: true},
					manual.Span{Role: manual.Muted, Text: " ["},
					manual.Span{Role: manual.Accent, Text: "<task|key>"},
					manual.Span{Role: manual.Muted, Text: " [args…]]"},
				),
				richBlock(4, false,
					manual.Span{Role: manual.Accent2, Text: "menu", Bold: true},
					manual.Span{Role: manual.Foreground, Text: " list"},
				),
				richBlock(4, false,
					manual.Span{Role: manual.Accent2, Text: "menu", Bold: true},
					manual.Span{Role: manual.Foreground, Text: " help"},
					manual.Span{Role: manual.Muted, Text: " | "},
					manual.Span{Role: manual.Accent2, Text: "docs", Bold: true},
					manual.Span{Role: manual.Muted, Text: " | "},
					manual.Span{Role: manual.Accent2, Text: "d", Bold: true},
					manual.Span{Role: manual.Muted, Text: "   this manual"},
				),
			},
		},
		{
			Title: "description",
			Blocks: []manual.Block{
				paragraph(manual.Muted, 4, project+" greets you with a static MOTD on shell entry, then exposes a small set of shortcuts for the rest of the session. The interactive menu is a fuzzy-filtered picker over the tasks declared in Nix: type to filter, ↑/↓ to select, ↵ to run, and ⇥ to expand details.", true),
				paragraph(manual.Muted, 4, "Tasks that declare arguments open argument-entry mode with suggested value chips, boolean flags, required-argument validation, and a live command preview. Extra command-line arguments bypass argument entry and are appended verbatim.", true),
			},
		},
		{Title: "options"},
		{Title: "commands"},
		{Title: "examples"},
		{Title: "see also"},
	}
	if cfg.Execute {
		sections[2].Blocks = append(sections[2].Blocks, paragraph(manual.Muted, 4, "The selected command replaces the menu process (bash -c) and the menu exits with its status.", false))
	} else {
		sections[2].Blocks = append(sections[2].Blocks, paragraph(manual.Muted, 4, "This menu is configured to print the assembled command instead of executing it.", false))
	}

	appendEntry(&sections[3], "help, ?", "Reprint the MOTD welcome banner.")
	appendEntry(&sections[3], "menu, m", "Open the interactive command picker. Pass a task name or key to run it directly.")
	appendEntry(&sections[3], "docs, d", "Open this manual (same as menu help). Digits 1–7 jump to sections; j/k scroll; q quits.")
	appendEntry(&sections[3], "--config <path>", "Path to the menu config JSON. Defaults to $PRELUDE_MENU_CONFIG; the Nix wrapper bakes this in.")
	appendEntry(&sections[3], "PRELUDE_MENU_DEBUG=<path>", "Write TUI diagnostics to the given file.")

	appendEntry(&sections[4], "list", "Print the grouped task table and exit (non-interactive).")
	appendEntry(&sections[4], "help", "Show this manual.")
	for _, group := range cfg.Groups {
		if group.Title != "" {
			sections[4].Blocks = append(sections[4].Blocks, richBlock(4, true,
				manual.Span{Role: manual.Muted, Text: strings.ToUpper(group.Title)},
			))
		}
		for _, task := range group.Tasks {
			spans := []manual.Span{{Role: manual.Accent2, Text: task.Name, Bold: true}}
			if task.Key != "" {
				spans = append(spans, manual.Span{Role: manual.Dim, Text: "  (" + task.Key + ")"})
			}
			sections[4].Blocks = append(sections[4].Blocks, richBlock(4, false, spans...))
			if task.Description != "" {
				sections[4].Blocks = append(sections[4].Blocks, paragraph(manual.Muted, 6, task.Description, false))
			}
			if task.Details != "" {
				sections[4].Blocks = append(sections[4].Blocks, paragraph(manual.Muted, 6, task.Details, false))
			}
			if task.Usage != "" {
				sections[4].Blocks = append(sections[4].Blocks, shellLine(6, task.Usage, false))
			}
			for _, example := range task.Examples {
				sections[4].Blocks = append(sections[4].Blocks, shellLine(6, example, false))
			}
			sections[4].Blocks = append(sections[4].Blocks, manual.Block{BlankAfter: true})
		}
	}

	appendExample := func(command, description string) {
		sections[5].Blocks = append(sections[5].Blocks, shellLine(4, command, false))
		if description != "" {
			sections[5].Blocks = append(sections[5].Blocks, paragraph(manual.Dim, 6, description, false))
		}
		sections[5].Blocks = append(sections[5].Blocks, manual.Block{BlankAfter: true})
	}
	appendExample("help", "reprint the welcome banner")
	appendExample("menu", "open the interactive picker")
	appendExample("menu list", "print the task table without a TTY")
	appendExample("docs", "open this manual; press 5 to jump to COMMANDS")
	for _, group := range cfg.Groups {
		for _, task := range group.Tasks {
			for _, example := range task.Examples {
				appendExample(example, "")
			}
		}
	}

	sections[6].Blocks = []manual.Block{
		richBlock(4, false, manual.Span{Role: manual.Foreground, Text: "motd"}, manual.Span{Role: manual.Muted, Text: " — static welcome banner (also: help, ?)"}),
		richBlock(4, false, manual.Span{Role: manual.Foreground, Text: "menu list"}, manual.Span{Role: manual.Muted, Text: " — the same catalogue as a plain table"}),
		richBlock(4, false, manual.Span{Role: manual.Foreground, Text: "⇥ in the picker"}, manual.Span{Role: manual.Muted, Text: " — per-task details, usage, and examples"}),
		richBlock(4, true, manual.Span{Role: manual.Foreground, Text: "README.md"}, manual.Span{Role: manual.Muted, Text: " — module options, themes, and downstream usage"}),
	}

	return manual.Document{Sections: sections}
}

func richBlock(indent int, blankAfter bool, spans ...manual.Span) manual.Block {
	return manual.Block{Indent: indent, BlankAfter: blankAfter, Spans: spans}
}

func paragraph(role manual.Role, indent int, text string, blankAfter bool) manual.Block {
	return manual.Block{Indent: indent, Wrap: true, BlankAfter: blankAfter, Spans: []manual.Span{{Role: role, Text: text}}}
}

func shellLine(indent int, command string, blankAfter bool) manual.Block {
	return richBlock(indent, blankAfter,
		manual.Span{Role: manual.Accent, Text: "$ "},
		manual.Span{Role: manual.Foreground, Text: command},
	)
}

func appendEntry(section *manual.Section, term, description string) {
	section.Blocks = append(section.Blocks,
		richBlock(4, false, manual.Span{Role: manual.Accent2, Text: term, Bold: true}),
		paragraph(manual.Muted, 6, description, true),
	)
}
