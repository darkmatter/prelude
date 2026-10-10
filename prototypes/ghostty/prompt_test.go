package main

import (
	"reflect"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestPromptSemanticLifecycleAcrossSplitControlStrings(t *testing.T) {
	var parser promptParser
	var state promptState
	frame := spikeFrame(40, 2)
	for i, char := range spikePrompt {
		frame.Cells[i] = uv.Cell{Content: string(char), Width: 1}
	}
	frame.CursorX = len(spikePrompt)
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
		frame.Cells[len(spikePrompt)+i] = uv.Cell{Content: string(char), Width: 1}
	}
	frame.CursorX = len(spikePrompt + "echo hello")
	state.observe(frame)
	if kinds != "AB" || state.phase != "prompt" || !state.ready || state.anchor != len(spikePrompt) ||
		state.anchorCols != frame.Cols || !state.promptAt(frame, state.anchor) {
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
	for i, char := range spikePrompt {
		frame.Cells[frame.Cols+i] = uv.Cell{Content: string(char), Width: 1}
	}
	frame.CursorX = len(spikePrompt)
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

func TestPromptAnchorSurvivesWrappedGraphemesAndSuffix(t *testing.T) {
	frame := spikeFrame(12, 3)
	for i, char := range spikePrompt + "ab" {
		frame.Cells[i] = uv.Cell{Content: string(char), Width: 1}
	}
	frame.Cells[10] = uv.Cell{Content: "界", Width: 2}
	frame.Cells[11] = uv.Cell{}
	frame.Cells[12] = uv.Cell{Content: "e\u0301", Width: 1}
	frame.Cells[13] = uv.Cell{Content: "x", Width: 1}
	frame.CursorX = len(spikePrompt)
	var state promptState
	state.mark(promptMarker{kind: 'B'}, frame)
	fingerprint := append([]promptCell(nil), state.fingerprint...)
	for _, cursor := range [][2]int{{9, 0}, {3, 1}} {
		frame.CursorX, frame.CursorY = cursor[0], cursor[1]
		before := frame
		before.Cells = append([]uv.Cell(nil), frame.Cells...)
		state.observe(frame)
		if state.anchor != len(spikePrompt) || !state.promptAt(frame, state.anchor) ||
			!reflect.DeepEqual(state.fingerprint, fingerprint) || !reflect.DeepEqual(frame, before) {
			t.Fatalf("middle edit or wrapped suffix changed the prompt boundary, fingerprint, or native cells: anchor=%d", state.anchor)
		}
	}
	frame.Cells[0].Content = "!"
	state.observe(frame)
	if state.anchor >= 0 {
		t.Fatalf("erased prompt retained a usable anchor: %d", state.anchor)
	}
	state.observe(frame)
	if state.anchor >= 0 {
		t.Fatal("an unknown anchor became valid without a matching prompt or new B")
	}
}

// Exercise OSC133 against actual native cells, not byte counts or shell source.
func feedPromptTest(t *testing.T, engine *terminal, state *promptState, parser *promptParser, data string) terminalFrame {
	t.Helper()
	from := 0
	for _, marker := range parser.scan([]byte(data)) {
		state.promptOutput([]byte(data[from:marker.end]))
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
	state.promptOutput([]byte(data[from:]))
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
	if !state.ready || state.anchor != 2*frame.Cols+5 || !state.promptAt(frame, state.anchor) ||
		state.promptRows != 3 || state.promptRowsCols != frame.Cols || state.promptBytes != nil {
		t.Fatalf("semantic B lost the arbitrary wide multiline prompt boundary: %+v", state)
	}
	fingerprint := append([]promptCell(nil), state.fingerprint...)
	frame = feedPromptTest(t, engine, &state, &parser, "\x1b[1S\x1b[1A")
	if state.anchor != frame.Cols+5 || !state.promptAt(frame, state.anchor) || !reflect.DeepEqual(state.fingerprint, fingerprint) {
		t.Fatalf("scroll lost the rendered wide/combining prompt fingerprint: anchor=%d", state.anchor)
	}
	if err := engine.Resize(8, 4); err != nil {
		t.Fatal(err)
	}
	frame = feedPromptTest(t, engine, &state, &parser, "")
	// A native resize can expose a sparse viewport until readline redraws.
	// A retained boundary is usable only if its complete fingerprint matches.
	if state.anchor >= 0 && (state.anchorCols != frame.Cols || !state.promptAt(frame, state.anchor) ||
		state.anchor > frame.CursorY*frame.Cols+frame.CursorX) {
		t.Fatalf("reflow retained an unsafe prompt boundary: anchor=%d cols=%d", state.anchor, state.anchorCols)
	}
	if !reflect.DeepEqual(state.fingerprint, fingerprint) {
		t.Fatal("reflow replaced the prompt fingerprint with editable input")
	}
	// Redraw recolors PS1 and wraps input; neither changes the semantic B.
	frame = feedPromptTest(t, engine, &state, &parser, "\x1b[2J\x1b[H\x1b]133;A\a\x1b[31m界e\u0301❯ \x1b[0m\x1b]133;B\aa界z")
	if state.anchor != 5 || state.anchorCols != 8 || !state.promptAt(frame, state.anchor) ||
		state.promptRows != 1 || state.promptRowsCols != 8 || !reflect.DeepEqual(state.fingerprint, fingerprint) {
		t.Fatalf("redrawn prompt changed the semantic input boundary: %+v", state)
	}
	if frame.Cells[5].Content != "a" || frame.Cells[6].Content != "界" || frame.Cells[6].Width != 2 ||
		frame.Cells[7].Width != 0 || frame.Cells[8].Content != "z" || frame.CursorX != 1 || frame.CursorY != 1 {
		t.Fatal("native redraw lost the wrapped editable graphemes or caret")
	}
}

func TestPromptUnknownAnchorSuppressesCompletion(t *testing.T) {
	engine := ghosttyTestTerminal(t, 80, 7)
	ghosttyWrite(t, engine, spikePrompt+"x go:")
	h := &host{cols: 80, rows: 8, terminal: engine, frame: ghosttySnapshot(t, engine), terminalFocused: true}
	h.prompt.mark(promptMarker{kind: 'B'}, terminalFrame{})
	h.prompt.observe(h.frame)
	h.completion = &completionState{phase: completionPopup, snapshot: completionSnapshot{candidates: []completionCandidate{
		{name: "go:test"}, {name: "go:vet"},
	}}}
	if !h.layout().Completion.Empty() {
		t.Fatal("unknown semantic boundary left a usable candidate area")
	}
	// Snapshot is the host boundary that dismisses a chooser after its anchor
	// becomes unknown. View only projects the resulting UI-loop state.
	if err := h.snapshot(); err != nil {
		t.Fatal(err)
	}
	layout := h.layout()
	view := h.View()
	rows := strings.Split(ansi.Strip(view.Content), "\n")
	if h.prompt.anchor >= 0 || h.completion != nil || !layout.Completion.Empty() || layout.ShellScroll != 0 || layout.FooterY != h.rows-1 ||
		len(rows) != h.rows || strings.TrimSpace(rows[0]) != spikePrompt+"x go:" ||
		strings.Contains(ansi.Strip(view.Content), completionHint) || strings.TrimRight(rows[layout.FooterY], " ") != baseFooter ||
		view.Cursor == nil || view.Cursor.X != h.frame.CursorX || view.Cursor.Y != h.frame.CursorY {
		t.Fatalf("unknown semantic boundary must not infer an anchor from prompt-like text or project a chooser: anchor=%d layout=%+v\n%s", h.prompt.anchor, layout, ansi.Strip(view.Content))
	}
}

func TestPromptSparseClippedAndRightEdgeBoundariesStayUnknown(t *testing.T) {
	var state promptState
	state.mark(promptMarker{kind: 'B'}, terminalFrame{})
	state.observe(spikeFrame(20, 2))
	if state.anchor >= 0 || len(state.fingerprint) != 0 {
		t.Fatalf("sparse B fabricated an editable boundary: %+v", state)
	}
	for _, test := range []struct {
		name   string
		cols   int
		prompt string
		resize int
	}{
		{"bounded tail", 400, strings.Repeat("x", 300), 399},
		{"ambiguous pending wrap", 8, spikePrompt, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, err := newTerminal(test.cols, 3)
			if err != nil {
				t.Fatal(err)
			}
			defer engine.Close()
			var state promptState
			var parser promptParser
			frame := feedPromptTest(t, engine, &state, &parser, "\x1b]133;A\a"+test.prompt+"\x1b]133;B\ahello")
			if test.resize != 0 {
				if state.anchor != len(test.prompt) || !state.promptAt(frame, state.anchor) ||
					len(state.fingerprint) != promptFingerprintCells || state.wholeTail {
					t.Fatalf("OSC133 B did not retain a bounded, non-relocatable prompt tail: %+v", state)
				}
				if err := engine.Resize(test.resize, 3); err != nil {
					t.Fatal(err)
				}
				feedPromptTest(t, engine, &state, &parser, "")
			}
			if state.anchor >= 0 {
				t.Fatalf("clipped/ambiguous prompt retained a usable input boundary: %d", state.anchor)
			}
		})
	}
}
