package motd

import (
	"testing"
	"time"
)

// Height-gated spacing: vertical sides vanish on short terminals, horizontal
// sides and ungated spacing stay put.
func TestSpacingCollapsesVerticalSidesBelowMinHeight(t *testing.T) {
	cfg := Config{
		Margin:  Spacing{Top: 2, Bottom: 10, Left: 3, Right: 1, MinHeight: 40},
		Padding: Spacing{Top: 1, Bottom: 2, Left: 4, MinHeight: 40},
	}

	short := newRenderer(Resolve(cfg, Cache{}, 80, 30, time.Now()))
	if short.model.Margin.Top != 0 || short.model.Margin.Bottom != 0 {
		t.Fatalf("short margin = %+v, want collapsed vertical sides", short.model.Margin)
	}
	if short.model.Margin.Left != 3 || short.model.Margin.Right != 1 || short.model.Padding.Left != 4 {
		t.Fatalf("horizontal sides changed: margin=%+v padding=%+v", short.model.Margin, short.model.Padding)
	}
	if short.model.Padding.Top != 0 || short.model.Padding.Bottom != 0 {
		t.Fatalf("short padding = %+v, want collapsed vertical sides", short.model.Padding)
	}

	tall := newRenderer(Resolve(cfg, Cache{}, 80, 40, time.Now()))
	if tall.model.Margin.Bottom != 10 || tall.model.Padding.Bottom != 2 {
		t.Fatalf("at-threshold spacing collapsed: margin=%+v padding=%+v", tall.model.Margin, tall.model.Padding)
	}

	ungated := newRenderer(Resolve(Config{Margin: Spacing{Bottom: 10}}, Cache{}, 80, 5, time.Now()))
	if ungated.model.Margin.Bottom != 10 {
		t.Fatalf("MinHeight=0 must never collapse: %+v", ungated.model.Margin)
	}
}
