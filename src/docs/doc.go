package main

import (
	"strings"

	"prelude/manual"
)

func manualDocument(cfg *Config) manual.Document {
	document := manual.Document{Sections: make([]manual.Section, 0, len(cfg.Sections))}
	for _, sourceSection := range cfg.Sections {
		section := manual.Section{Title: sourceSection.Title}
		for _, sourceBlock := range sourceSection.Blocks {
			switch strings.ToLower(sourceBlock.Type) {
			case "lead":
				term := sourceBlock.Term
				if term == "" {
					term = cfg.Project
				}
				spans := []manual.Span{{Role: manual.Accent2, Text: term, Bold: true}}
				if sourceBlock.Text != "" {
					spans = append(spans, manual.Span{Role: manual.Foreground, Text: " — " + sourceBlock.Text})
				}
				section.Blocks = append(section.Blocks, manual.Block{Indent: 4, Spans: spans, BlankAfter: true})

			case "para", "paragraph":
				section.Blocks = append(section.Blocks, paragraphBlock(manual.Muted, 4, sourceBlock.Text, true))

			case "option", "command":
				term := sourceBlock.Term
				if term == "" {
					term = sourceBlock.Command
				}
				if term != "" {
					section.Blocks = append(section.Blocks, manual.Block{
						Indent: 4,
						Spans:  []manual.Span{{Role: manual.Accent2, Text: term, Bold: true}},
					})
				}
				section.Blocks = append(section.Blocks, paragraphBlock(manual.Muted, 6, sourceBlock.Text, true))

			case "shell", "example":
				command := sourceBlock.Command
				if command == "" {
					command = sourceBlock.Text
				}
				if command != "" {
					section.Blocks = append(section.Blocks, shellBlock(4, command, false))
				}
				note := sourceBlock.Note
				if note == "" && strings.EqualFold(sourceBlock.Type, "example") && sourceBlock.Text != "" && sourceBlock.Command != "" {
					note = sourceBlock.Text
				}
				if note != "" {
					section.Blocks = append(section.Blocks, paragraphBlock(manual.Dim, 6, note, true))
				} else {
					section.Blocks = append(section.Blocks, manual.Block{BlankAfter: true})
				}

			case "blank":
				section.Blocks = append(section.Blocks, manual.Block{BlankAfter: true})

			default:
				if sourceBlock.Text != "" {
					section.Blocks = append(section.Blocks, paragraphBlock(manual.Muted, 4, sourceBlock.Text, true))
				}
			}
		}
		document.Sections = append(document.Sections, section)
	}
	return document
}

func paragraphBlock(role manual.Role, indent int, text string, blankAfter bool) manual.Block {
	return manual.Block{
		Indent:     indent,
		Wrap:       true,
		BlankAfter: blankAfter,
		Spans:      []manual.Span{{Role: role, Text: text}},
	}
}

func shellBlock(indent int, command string, blankAfter bool) manual.Block {
	return manual.Block{
		Indent:     indent,
		BlankAfter: blankAfter,
		Spans: []manual.Span{
			{Role: manual.Accent, Text: "$ "},
			{Role: manual.Foreground, Text: command},
		},
	}
}
