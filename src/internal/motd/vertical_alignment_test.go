package motd

import "testing"

func TestVerticalFillRowsKeepsUnusedRowsInTheSelectedBand(t *testing.T) {
	cases := []struct {
		name      string
		align     string
		wantAbove int
		wantBelow int
	}{
		{name: "top", align: "top", wantAbove: 0, wantBelow: 5},
		{name: "center", align: "center", wantAbove: 2, wantBelow: 3},
		{name: "bottom", align: "bottom", wantAbove: 5, wantBelow: 0},
		{name: "empty preserves bottom", align: "", wantAbove: 5, wantBelow: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			above, below := verticalFillRows(tc.align, 5)
			if above != tc.wantAbove || below != tc.wantBelow {
				t.Fatalf("verticalFillRows(%q, 5) = (%d, %d), want (%d, %d)", tc.align, above, below, tc.wantAbove, tc.wantBelow)
			}
		})
	}
}

func TestVerticalFillRowsDoesNotCreateNegativeRows(t *testing.T) {
	above, below := verticalFillRows("center", -1)
	if above != 0 || below != 0 {
		t.Fatalf("verticalFillRows(center, -1) = (%d, %d), want (0, 0)", above, below)
	}
}
