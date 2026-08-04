package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const panelHelperEnv = "PRELUDE_PANEL_TEST_HELPER"

// runPanelHelper captures this test binary re-invoked as a panel command, so
// the assertions run against the real PTY capture path rather than a stub.
func runPanelHelper(t *testing.T, mode string, cols, rows int) *surface {
	t.Helper()
	captured, err := capturePanel(panelCapture{
		name: os.Args[0],
		args: []string{"-test.run=^TestPanelHelperProcess$"},
		cols: cols,
		rows: rows,
		env:  append(os.Environ(), panelHelperEnv+"="+mode),
	})
	if err != nil {
		t.Fatalf("capture panel: %v", err)
	}
	if captured == nil {
		t.Fatal("capture returned no surface")
	}
	return captured
}

// TestPanelHelperProcess is not a test: it is the child that TestPanel* runs.
// The two modes cannot be one program, because the repaint mode's erase would
// wipe the very text the info mode exists to report.
func TestPanelHelperProcess(t *testing.T) {
	switch os.Getenv(panelHelperEnv) {
	case "info":
		fmt.Printf("tty:%t\r\n", term.IsTerminal(int(os.Stdout.Fd())))
		fmt.Printf("cols:%s lines:%s\r\n", os.Getenv("COLUMNS"), os.Getenv("LINES"))
		fmt.Print("\x1b[31mRED\x1b[0m\r\n")

	case "repaint":
		// Address a cell, then erase the display and paint elsewhere. A
		// transcript-based capture would keep the erased text; a terminal
		// image must not.
		fmt.Print("GONE\r\n\x1b[2J\x1b[3;1HPAINTED")

	default:
		t.Skip("helper process; driven by the panel capture tests")
		return
	}
	os.Exit(0)
}

func TestPanelCaptureGivesTheCommandARealTerminal(t *testing.T) {
	// Commands that gate colour on isatty must see a TTY, or the pinned band
	// renders as the plain-text fallback nobody wants to look at.
	captured := runPanelHelper(t, "info", 40, 6)

	if text := surfaceText(captured); !strings.Contains(text, "tty:true") {
		t.Fatalf("panel child did not get a TTY:\n%s", text)
	}
}

func TestPanelCaptureTellsTheCommandItsGeometry(t *testing.T) {
	captured := runPanelHelper(t, "info", 37, 5)

	text := surfaceText(captured)
	if !strings.Contains(text, "cols:37") || !strings.Contains(text, "lines:5") {
		t.Fatalf("panel child was not told its geometry:\n%s", text)
	}
}

func TestPanelCapturePreservesStyling(t *testing.T) {
	// The reason panels go through a virtual terminal instead of ansi.Strip.
	captured := runPanelHelper(t, "info", 40, 6)

	found := false
	for y := range captured.rows {
		for x := range captured.cols {
			cell := captured.at(x, y)
			if cell != nil && cell.Content == "R" && cell.Style.Fg == ansi.Red {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("the child's red did not survive the capture:\n%s", surfaceText(captured))
	}
}

func TestPanelCaptureIsOpaqueEndToEnd(t *testing.T) {
	// The pin band sits over the child shell, so every cell of a captured
	// panel has to carry the band's own background. A cell with no background
	// resolves against the outer terminal's palette once the host composes it,
	// and the user's terminal theme shows through the chrome.
	captured := runPanelHelper(t, "info", 40, 6)

	for y := range captured.rows {
		for x := range captured.cols {
			cell := captured.at(x, y)
			if cell == nil {
				t.Fatalf("cell %d,%d is missing from the capture", x, y)
			}
			if cell.Width == 0 {
				continue // trailing half of a wide grapheme
			}
			if cell.Style.Bg != pinBackground {
				t.Fatalf("cell %d,%d (%q) background = %v, want the opaque pin background %v",
					x, y, cell.Content, cell.Style.Bg, pinBackground)
			}
		}
	}
}

func TestPanelCaptureKeepsThePictureNotTheTranscript(t *testing.T) {
	// The helper prints GONE, then erases the display and paints elsewhere.
	// A capture that concatenated output would still show GONE.
	captured := runPanelHelper(t, "repaint", 40, 6)

	text := surfaceText(captured)
	if strings.Contains(text, "GONE") {
		t.Fatalf("erased text survived the capture:\n%s", text)
	}
	if !strings.Contains(text, "PAINTED") {
		t.Fatalf("post-erase repaint was lost:\n%s", text)
	}
}

func TestPanelCaptureIsExactlyTheRequestedSize(t *testing.T) {
	// The host blits this straight into the pin band; a surface of the wrong
	// shape would either clip content or leave stale cells behind.
	captured := runPanelHelper(t, "info", 33, 7)

	if captured.cols != 33 || captured.rows != 7 {
		t.Fatalf("captured surface is %dx%d, want 33x7", captured.cols, captured.rows)
	}
}

func TestPanelCaptureReportsAMissingCommand(t *testing.T) {
	_, err := capturePanel(panelCapture{
		name: "prelude-command-that-does-not-exist",
		cols: 20,
		rows: 3,
		env:  os.Environ(),
	})
	if err == nil {
		t.Fatal("a missing command must be reported, not silently blank")
	}
	if !strings.Contains(err.Error(), "PATH") {
		t.Fatalf("error %q does not say why the command could not run", err)
	}
}

func TestPanelCaptureReturnsPartialOutputOnFailure(t *testing.T) {
	// A command that prints and then exits nonzero still has something worth
	// showing; the operator sees real output plus the error in the status row.
	captured, err := capturePanel(panelCapture{
		name: "/bin/sh",
		args: []string{"-c", "printf 'partial-output\\r\\n'; exit 9"},
		cols: 30,
		rows: 3,
		env:  os.Environ(),
	})
	if err == nil {
		t.Fatal("a nonzero exit must be reported")
	}
	if captured == nil {
		t.Fatal("output printed before the failure was discarded")
	}
	if text := surfaceText(captured); !strings.Contains(text, "partial-output") {
		t.Fatalf("partial output was lost:\n%s", text)
	}
}

func TestCapturePathRejectsLiveModes(t *testing.T) {
	// A live mode is a running child the host owns. Reaching the capture path
	// with one would mean photographing a program that is supposed to be
	// interactive, so it is a wiring bug rather than a fallback.
	for _, mode := range []pinMode{pinOff, pinMotd, pinMenu, pinDocs} {
		if !mode.live() {
			continue
		}
		if _, err := loadPanel(panelRequest{mode: mode, cols: 60, rows: 8}, os.Environ()); err == nil {
			t.Fatalf("loadPanel accepted the live mode %q", mode.label())
		}
	}
}

func TestDocsIsALiveMode(t *testing.T) {
	// The pane machinery keys off this, and a silent flip back to captured
	// would turn the docs pin into an unusable photograph again.
	if !pinDocs.live() {
		t.Fatal("docs must be a live mode so it can be navigated")
	}
	for _, mode := range []pinMode{pinOff, pinMotd, pinMenu} {
		if mode.live() {
			t.Fatalf("%q must stay a captured mode", mode.label())
		}
	}
}

// surfaceText flattens a captured surface for assertions.
func surfaceText(captured *surface) string {
	var out strings.Builder
	for y := range captured.rows {
		var row strings.Builder
		for x := range captured.cols {
			if cell := captured.at(x, y); cell != nil {
				row.WriteString(cell.Content)
			}
		}
		out.WriteString(strings.TrimRight(row.String(), " "))
		out.WriteByte('\n')
	}
	return out.String()
}
