package manual

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/shared"
)

// LeafCount returns the number of document leaves in depth-first order.
func LeafCount(document Document) int {
	return len(flattenLeaves(document.Nav))
}

// RenderLeafLines renders one leaf for print mode. Unlike the TUI body it
// omits the seed row and trailing body-fill cells so output can sit above the
// shell prompt without owning the terminal background.
func RenderLeafLines(document Document, palette shared.Palette, page, width int) ([]string, error) {
	leaves := flattenLeaves(document.Nav)
	if page < 1 || page > len(leaves) {
		return nil, fmt.Errorf("page %d out of range (1-%d)", page, len(leaves))
	}

	viewer := New(document, palette)
	leaf := leaves[page-1]
	var rendered []string
	if leaf.RootReadme {
		rendered = viewer.renderRootReadme(leaf, max(width, 24))
	} else {
		rendered = viewer.renderLeaf(leaf, max(width, 24))
	}
	if len(rendered) > 0 {
		rendered = rendered[1:]
	}
	for i, line := range rendered {
		plainWidth := ansi.StringWidth(strings.TrimRight(ansi.Strip(line), " \t"))
		rendered[i] = ansi.Truncate(line, plainWidth, "")
	}
	return rendered, nil
}

func flattenLeaves(nodes []NavNode) []*NavNode {
	var leaves []*NavNode
	var visit func([]NavNode)
	visit = func(nodes []NavNode) {
		for i := range nodes {
			node := &nodes[i]
			if node.IsGroup() {
				visit(node.Children)
				continue
			}
			leaves = append(leaves, node)
		}
	}
	visit(nodes)
	return leaves
}
