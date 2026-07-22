package manual

// Role selects a palette role for a document span.
type Role uint8

const (
	Foreground Role = iota
	Muted
	Dim
	Accent
	Accent2
)

// NavNode is one entry in the docs tree. Leaves hold Markdown; groups hold
// children. Groups never render a body.
type NavNode struct {
	Title      string
	Markdown   string
	Children   []NavNode
	GapBefore  bool // blank separator above this row (Options group)
	RootReadme bool // leaf path matched prelude.docs.rootReadme in Nix
}

// IsGroup reports whether the node is a non-leaf.
func (n NavNode) IsGroup() bool {
	return len(n.Children) > 0
}

// Document is the presentation model consumed by the viewer.
type Document struct {
	// Project is prelude.project; used for the root-README hero title when
	// no FIGlet hero is present, or as a fallback when the hero won't fit.
	Project string
	// Hero is the build-time FIGlet wordmark for the root-README page, or
	// empty when none was baked. The viewer renders it when it fits the
	// text width, else falls back to Project.
	Hero string
	// Nav is the docs tree. Leaves carry Markdown; groups carry children.
	Nav []NavNode
}

// SidebarLabel is the CONTENTS-column heading for the docs viewer.
func (d Document) SidebarLabel() string {
	return "PAGES"
}

// ModeLabel is the status-bar mode chip for the docs viewer.
func (d Document) ModeLabel() string {
	return "DOCS"
}

// SidebarItemsTop is the terminal row occupied by the first sidebar item.
const SidebarItemsTop = 4
