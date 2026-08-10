package main

import "testing"

func TestComputeLayoutFillsTheFrameExactly(t *testing.T) {
	// The three bands must tile the frame with no gap and no overlap, or the
	// composer leaves stale cells on screen.
	cases := []struct {
		name       string
		cols, rows int
		wantShell  int
		pin        pinMode
	}{
		{"pin off", 80, 24, 10, pinOff},
		{"pin on", 80, 24, 10, pinMotd},
		{"one row shell", 100, 30, 1, pinDocs},
		{"shell larger than frame", 80, 24, 999, pinMenu},
		{"minimum frame", 1, 3, 10, pinMenu},
		{"below minimum frame", 0, 0, 4, pinMotd},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			layout := computeLayout(testCase.cols, testCase.rows, testCase.wantShell, testCase.pin)

			if layout.pin.top != 0 {
				t.Fatalf("pin must start at row 0, got %d", layout.pin.top)
			}
			if layout.shell.top != layout.pin.bottom() {
				t.Fatalf("shell starts at %d, pin ends at %d", layout.shell.top, layout.pin.bottom())
			}
			if layout.statusRow != layout.shell.bottom() {
				t.Fatalf("status row %d, shell ends at %d", layout.statusRow, layout.shell.bottom())
			}
			if layout.statusRow != layout.rows-1 {
				t.Fatalf("status row %d is not the last row of %d", layout.statusRow, layout.rows)
			}
			if layout.shell.height < 1 {
				t.Fatalf("shell must keep at least one row, got %d", layout.shell.height)
			}
		})
	}
}

func TestComputeLayoutHonoursRequestedShellRows(t *testing.T) {
	layout := computeLayout(80, 24, 10, pinMotd)
	if layout.shell.height != 10 {
		t.Fatalf("shell height = %d, want 10", layout.shell.height)
	}
	if layout.pin.height != 13 {
		t.Fatalf("pin height = %d, want 13", layout.pin.height)
	}
}

func TestComputeLayoutGivesThePinNoRowsWhenOff(t *testing.T) {
	layout := computeLayout(80, 24, 10, pinOff)
	if layout.pin.height != 0 {
		t.Fatalf("pin height = %d, want 0 when the pin is off", layout.pin.height)
	}
	if layout.shell.height != 23 {
		t.Fatalf("shell height = %d, want the whole frame minus the status row", layout.shell.height)
	}
}

func TestPanelBodyExcludesTheHeaderRow(t *testing.T) {
	layout := computeLayout(80, 24, 10, pinMenu)
	body := layout.panelBody()

	if body.top != layout.pin.top+1 {
		t.Fatalf("panel body starts at %d, want one row below the pin top", body.top)
	}
	if body.bottom() != layout.pin.bottom() {
		t.Fatalf("panel body ends at %d, pin ends at %d", body.bottom(), layout.pin.bottom())
	}
}

func TestPanelBodyIsEmptyWhenOnlyTheHeaderFits(t *testing.T) {
	// A 3-row frame leaves one pin row, one shell row, one status row. There
	// is nowhere to put a capture, and a zero-row PTY must never be spawned.
	layout := computeLayout(80, 3, 10, pinMotd)
	if layout.pin.height != 1 {
		t.Fatalf("pin height = %d, want 1", layout.pin.height)
	}
	if got := layout.panelBody().height; got != 0 {
		t.Fatalf("panel body height = %d, want 0", got)
	}
}

func TestPinModeCyclesThroughEveryModeAndWraps(t *testing.T) {
	seen := []pinMode{pinOff}
	for mode := pinOff.next(); mode != pinOff; mode = mode.next() {
		seen = append(seen, mode)
		if len(seen) > len(pinLabels) {
			t.Fatal("pin cycle never returned to off")
		}
	}
	if len(seen) != len(pinLabels) {
		t.Fatalf("cycle visited %d modes, want %d", len(seen), len(pinLabels))
	}

	want := []pinMode{pinOff, pinMotd, pinMenu, pinDocs}
	wantLabels := []string{"off", "motd", "x", "docs"}
	for index, mode := range want {
		if seen[index] != mode {
			t.Fatalf("cycle position %d = %q, want %q", index, seen[index].label(), mode.label())
		}
		if mode.label() != wantLabels[index] {
			t.Fatalf("label for cycle position %d = %q, want %q", index, mode.label(), wantLabels[index])
		}
	}
}
