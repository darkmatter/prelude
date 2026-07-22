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
	return manual.Document{
		Project: cfg.Project,
		Hero:    cfg.Hero,
		Nav:     convertNav(cfg.Nav),
	}
}

func convertNav(nodes []NavNode) []manual.NavNode {
	out := make([]manual.NavNode, 0, len(nodes))
	for i, n := range nodes {
		out = append(out, convertNode(n, i))
	}
	return out
}

func convertNode(n NavNode, index int) manual.NavNode {
	if len(n.Children) > 0 || n.Kind == "group" {
		return manual.NavNode{
			Title:     n.Title,
			Children:  convertNav(n.Children),
			GapBefore: n.GapBefore,
		}
	}
	title := strings.TrimSpace(n.Title)
	if title == "" {
		title = markdownTitle(n.Markdown, index)
	}
	if n.RootReadme && title == "" {
		title = "README"
	}
	return manual.NavNode{
		Title:      title,
		Markdown:   n.Markdown,
		GapBefore:  n.GapBefore,
		RootReadme: n.RootReadme,
	}
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
