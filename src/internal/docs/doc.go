package docs

import (
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"prelude/pkg/manual"
)

func manualDocument(cfg *Config) manual.Document {
	document := manual.Document{
		Kind:     manual.KindDocs,
		Sections: make([]manual.Section, 0, len(cfg.Pages)),
	}
	for index, page := range cfg.Pages {
		document.Sections = append(document.Sections, manual.Section{
			Title:    markdownTitle(page.Text, index),
			Markdown: page.Text,
		})
	}
	return document
}

func markdownTitle(source string, index int) string {
	contents := []byte(source)
	root := goldmark.DefaultParser().Parse(text.NewReader(contents))
	for node := root.FirstChild(); node != nil; node = node.NextSibling() {
		heading, ok := node.(*ast.Heading)
		if !ok || heading.Level != 1 {
			continue
		}
		if title := strings.TrimSpace(string(heading.Text(contents))); title != "" {
			return title
		}
	}
	return fmt.Sprintf("page %d", index+1)
}
