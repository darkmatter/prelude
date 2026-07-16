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

// Span is one styled fragment in a document block.
type Span struct {
	Role Role
	Text string
	Bold bool
}

// Block is one semantic content row. Wrapped blocks should contain one span.
type Block struct {
	Indent     int
	Wrap       bool
	BlankAfter bool
	Spans      []Span
}

// Section is one sidebar entry and its content. Markdown is used by the docs
// viewer; Blocks remain available for structured manuals such as menu help.
type Section struct {
	Title    string
	Markdown string
	Blocks   []Block
}

// Document is the presentation model consumed by the viewer.
type Document struct {
	Sections []Section
}

// SidebarItemsTop is the terminal row occupied by the first sidebar item.
const SidebarItemsTop = 4
