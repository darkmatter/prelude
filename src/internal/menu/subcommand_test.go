package menu

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFlattenSearchesModuleChildrenThroughParent(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name:  "e2e",
		Label: "e2e",
		Children: []Task{
			{Name: "e2e::desktop", Label: "desktop", Description: "run desktop e2e"},
		},
	})
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.prompt = m.prompt.WithValue("desktop")
	m.filter()

	if len(m.matches) != 1 || m.flat[m.matches[0]].Name != "e2e" {
		t.Fatalf("matches = %#v, want the e2e parent", m.matches)
	}
}

// rowIndent returns the leading-space count of the first line containing row.
func rowIndent(rendered, row string) int {
	for _, line := range strings.Split(rendered, "\n") {
		if strings.Contains(line, row) {
			return len(line) - len(strings.TrimLeft(line, " "))
		}
	}
	return -1
}

func TestPrintListIndentsModuleChildren(t *testing.T) {
	cfg := testMenuConfig(Task{
		Name:        "e2e",
		Label:       "e2e",
		Description: "2 subcommands",
		Children: []Task{
			{Name: "e2e::coder", Label: "coder", Description: "run coder e2e"},
			{Name: "e2e::desktop", Label: "desktop", Description: "run desktop e2e", Args: []Arg{{Token: "file"}}},
		},
	})

	var out strings.Builder
	printListTo(&out, nil, cfg, newStyles(cfg, false))
	rendered := ansi.Strip(out.String())

	if !strings.Contains(rendered, "e2e 2 subcommands") {
		t.Fatalf("module parent row missing from --list output: %q", rendered)
	}
	if !strings.Contains(rendered, "coder run coder e2e") || !strings.Contains(rendered, "desktop run desktop e2e") {
		t.Fatalf("subcommand children missing from --list output: %q", rendered)
	}
	if !strings.Contains(rendered, "▸ 2") {
		t.Fatalf("subcommand count marker missing from --list output: %q", rendered)
	}
	if !strings.Contains(rendered, "◆ args") {
		t.Fatalf("args marker missing for the desktop child: %q", rendered)
	}
	// Children render one level deeper than their parent row.
	parentIndent := rowIndent(rendered, "e2e 2 subcommands")
	childIndent := rowIndent(rendered, "desktop run desktop e2e")
	if parentIndent < 0 || childIndent < 0 {
		t.Fatalf("rows missing from --list output: %q", rendered)
	}
	if childIndent <= parentIndent {
		t.Fatalf("child row indent = %d, want deeper than parent indent %d", childIndent, parentIndent)
	}
}

func TestFlattenSearchesNestedSubcommandsThroughEachParent(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)
	m := newModel(cfg, newStyles(cfg, false), nil)
	m.prompt = m.prompt.WithValue("orders")
	m.filter()
	if len(m.matches) != 1 || selectedName(m) != "db" {
		t.Fatalf("root matches = %v, want the db row", m.matches)
	}

	// The picker's rows come straight from the Config; they search too.
	m.enterSubMode(m.flat[m.matches[0]])
	m.prompt = m.prompt.WithValue("orders")
	m.filter()
	if len(m.matches) != 1 || selectedName(m) != "db seed" {
		t.Fatalf("db picker matches = %v, want the seed row", m.matches)
	}
}

func TestPrintListIndentsEveryDepth(t *testing.T) {
	cfg := testMenuConfig(subcommandTestTasks()...)

	var out strings.Builder
	printListTo(&out, nil, cfg, newStyles(cfg, false))
	rendered := ansi.Strip(out.String())

	indents := []int{
		rowIndent(rendered, "db 2 subcommands"),
		rowIndent(rendered, "seed 2 subcommands"),
		rowIndent(rendered, "orders "),
	}
	if indents[0] < 0 || indents[0] >= indents[1] || indents[1] >= indents[2] {
		t.Fatalf("rows should indent one level per depth, got %v:\n%s", indents, rendered)
	}
	if rowIndent(rendered, "migrate ") != indents[1] || rowIndent(rendered, "staging ") != indents[1] {
		t.Fatalf("siblings should share an indent:\n%s", rendered)
	}
}
