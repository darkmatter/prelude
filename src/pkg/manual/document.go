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

// Kind distinguishes the two surfaces that share this viewer so chrome can
// label them differently. Content sources stay separate: authored Markdown
// pages (docs) versus the generated command manual (help).
type Kind uint8

const (
	// KindHelp is menu help — a generated man-style command manual.
	KindHelp Kind = iota
	// KindDocs is the project docs viewer — Markdown from prelude.docs.pages.
	KindDocs
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
	Kind     Kind
	Sections []Section
}

// SidebarLabel is the CONTENTS-column heading for this document kind.
func (d Document) SidebarLabel() string {
	if d.Kind == KindDocs {
		return "PAGES"
	}
	return "MANUAL"
}

// ModeLabel is the status-bar mode chip for this document kind.
func (d Document) ModeLabel() string {
	if d.Kind == KindDocs {
		return "DOCS"
	}
	return "HELP"
}

// SidebarItemsTop is the terminal row occupied by the first sidebar item.
const SidebarItemsTop = 4
