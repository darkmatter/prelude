package main

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestPromptSemanticLifecycleAcrossSplitControlStrings(t *testing.T) {
	var parser promptParser
	var state promptState
	frame := spikeFrame(40, 2)
	for i, char := range workspacePrompt {
		frame.Cells[i] = uv.Cell{Content: string(char), Width: 1}
	}
	frame.CursorX = len(workspacePrompt)
	stream := "\x1bPignored \x1b]133;C\a\x1b\\" +
		"\x1b]0;not a shell hook\a\x1b]133;A\a\x1b]133;B\x1b\\"
	var kinds string
	// Each marker can be split at every byte, including between ESC and ST.
	for i := range len(stream) {
		for _, marker := range parser.scan([]byte(stream[i : i+1])) {
			kinds += string(marker.kind)
			state.mark(marker, frame)
		}
	}
	for i, char := range "echo hello" {
		frame.Cells[len(workspacePrompt)+i] = uv.Cell{Content: string(char), Width: 1}
	}
	frame.CursorX = len(workspacePrompt + "echo hello")
	state.observe(frame)
	if kinds != "AB" || state.phase != "prompt" || !state.ready || state.anchor != len(workspacePrompt) ||
		state.anchorCols != frame.Cols {
		t.Fatalf("hooks=%q phase=%q ready=%t anchor=%d cols=%d", kinds, state.phase, state.ready, state.anchor, state.anchorCols)
	}
	frame.CursorX, frame.CursorY = 0, 1
	for _, marker := range parser.scan([]byte("\x1b]133;C\a")) {
		state.mark(marker, frame)
	}
	if state.phase != "running" || state.ready {
		t.Fatalf("submitted lifecycle: phase=%q ready=%t", state.phase, state.ready)
	}
	markers := parser.scan([]byte("\x1b]133;D;23\a"))
	if len(markers) != 1 || markers[0].kind != 'D' || markers[0].status != 23 {
		t.Fatalf("standard OSC133 exit payload = %+v", markers)
	}
	state.mark(markers[0], frame)
	if state.phase != "prompt" || state.ready {
		t.Fatalf("D alone made input ready: phase=%q ready=%t", state.phase, state.ready)
	}
	state.mark(promptMarker{kind: 'A'}, frame)
	if state.ready || state.anchor >= 0 {
		t.Fatal("A must invalidate the previous editable boundary until B")
	}
	for i, char := range workspacePrompt {
		frame.Cells[frame.Cols+i] = uv.Cell{Content: string(char), Width: 1}
	}
	frame.CursorX = len(workspacePrompt)
	state.mark(promptMarker{kind: 'B'}, frame)
	if !state.ready || state.anchor != frame.CursorY*frame.Cols+frame.CursorX {
		t.Fatal("B did not establish the new semantic boundary")
	}
	if markers := parser.scan([]byte("\x1b]" + strings.Repeat("x", 10000) + "\a")); len(markers) != 0 || parser.n != 0 {
		t.Fatal("oversized unrelated OSC was not bounded and discarded")
	}
}

func TestPromptParserStatusPayload(t *testing.T) {
	for _, tc := range []struct {
		payload string
		want    int
	}{
		{"", -1}, {";0", 0}, {";23", 23}, {";255", 255},
		{";-1", -1}, {";256", -1}, {";invalid", -1},
	} {
		t.Run(tc.payload, func(t *testing.T) {
			var parser promptParser
			markers := parser.scan([]byte("\x1b]133;D" + tc.payload + "\x1b\\"))
			if len(markers) != 1 || markers[0].kind != 'D' || markers[0].status != tc.want {
				t.Fatalf("OSC133 D payload %q = %+v, want status %d", tc.payload, markers, tc.want)
			}
		})
	}
}

// Exercise OSC133 against actual native cells, not byte counts or shell source.
func feedPromptTest(t *testing.T, engine *terminal, state *promptState, parser *promptParser, data string) terminalFrame {
	t.Helper()
	from := 0
	for _, marker := range parser.scan([]byte(data)) {
		if _, err := engine.Write([]byte(data[from:marker.end])); err != nil {
			t.Fatal(err)
		}
		frame, err := engine.Snapshot()
		if err != nil {
			t.Fatal(err)
		}
		state.mark(marker, frame)
		from = marker.end
	}
	if _, err := engine.Write([]byte(data[from:])); err != nil {
		t.Fatal(err)
	}
	frame, err := engine.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	state.observe(frame)
	return frame
}

func TestPromptSemanticBoundaryForColoredMultilineWidePrompt(t *testing.T) {
	engine, err := newTerminal(16, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	var state promptState
	var parser promptParser
	feedPromptTest(t, engine, &state, &parser, "\x1b]133;A\aheader\r\n│\r\n\x1b[32m界e\u0301❯ \x1b[0m\x1b]133;B\a")
	frame := feedPromptTest(t, engine, &state, &parser, "a界z")
	if !state.ready || state.anchor != 2*frame.Cols+5 {
		t.Fatalf("semantic B lost the arbitrary wide multiline prompt boundary: %+v", state)
	}
	if err := engine.Resize(8, 4); err != nil {
		t.Fatal(err)
	}
	// Geometry changed without a new B: the old boundary is unknown, not relocated.
	frame = feedPromptTest(t, engine, &state, &parser, "")
	if state.anchor >= 0 {
		t.Fatalf("reflow retained a prompt boundary across widths: anchor=%d cols=%d", state.anchor, state.anchorCols)
	}
	// Redraw recolors PS1 and wraps input; neither changes the semantic B.
	frame = feedPromptTest(t, engine, &state, &parser, "\x1b[2J\x1b[H\x1b]133;A\a\x1b[31m界e\u0301❯ \x1b[0m\x1b]133;B\aa界z")
	if state.anchor != 5 || state.anchorCols != 8 {
		t.Fatalf("redrawn prompt changed the semantic input boundary: %+v", state)
	}
	if frame.Cells[5].Content != "a" || frame.Cells[6].Content != "界" || frame.Cells[6].Width != 2 ||
		frame.Cells[7].Width != 0 || frame.Cells[8].Content != "z" || frame.CursorX != 1 || frame.CursorY != 1 {
		t.Fatal("native redraw lost the wrapped editable graphemes or caret")
	}
}

func TestPromptRecapturesAfterMarkerWhileScrolled(t *testing.T) {
	engine := ghosttyTestTerminal(t, 80, 7)
	ghosttyWrite(t, engine, workspacePrompt+"x go:")
	live := ghosttySnapshot(t, engine)
	scrolled := live
	scrolled.Scrolled = true

	var state promptState
	state.mark(promptMarker{kind: 'B'}, scrolled)
	if state.anchor != -1 {
		t.Fatalf("scrolled marker captured a live-screen anchor: %+v", state)
	}
	// Output can keep arriving while the user reads history; the first
	// live snapshot must restore the semantic input boundary.
	state.observe(live)
	if want := live.CursorY*live.Cols + live.CursorX; state.anchor != want {
		t.Fatalf("anchor = %d, want %d after returning to the live screen", state.anchor, want)
	}
}

func TestPromptSparseAndRightEdgeBoundariesStayUnknown(t *testing.T) {
	var state promptState
	state.mark(promptMarker{kind: 'B'}, terminalFrame{})
	state.observe(spikeFrame(20, 2))
	if state.anchor >= 0 {
		t.Fatalf("sparse B fabricated an editable boundary: %+v", state)
	}
	engine, err := newTerminal(len(workspacePrompt), 3)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	var parser promptParser
	feedPromptTest(t, engine, &state, &parser, "\x1b]133;A\a"+workspacePrompt+"\x1b]133;B\ahello")
	if state.anchor >= 0 {
		t.Fatalf("ambiguous pending-wrap prompt retained a usable input boundary: %d", state.anchor)
	}
}
