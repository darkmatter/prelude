package motd

import "testing"

// Height-gated spacing: vertical sides vanish on short terminals, horizontal
// sides and ungated spacing stay put.
func TestSpacingCollapsesVerticalSidesBelowMinHeight(t *testing.T) {
	cfg := Config{
		Margin:  Spacing{Top: 2, Bottom: 10, Left: 3, Right: 1, MinHeight: 40},
		Padding: Spacing{Top: 1, Bottom: 2, Left: 4, MinHeight: 40},
	}

	short := newRenderer(cfg, 80, 30)
	if short.cfg.Margin.Top != 0 || short.cfg.Margin.Bottom != 0 {
		t.Fatalf("short margin = %+v, want collapsed vertical sides", short.cfg.Margin)
	}
	if short.cfg.Margin.Left != 3 || short.cfg.Margin.Right != 1 || short.cfg.Padding.Left != 4 {
		t.Fatalf("horizontal sides changed: margin=%+v padding=%+v", short.cfg.Margin, short.cfg.Padding)
	}
	if short.cfg.Padding.Top != 0 || short.cfg.Padding.Bottom != 0 {
		t.Fatalf("short padding = %+v, want collapsed vertical sides", short.cfg.Padding)
	}

	tall := newRenderer(cfg, 80, 40)
	if tall.cfg.Margin.Bottom != 10 || tall.cfg.Padding.Bottom != 2 {
		t.Fatalf("at-threshold spacing collapsed: margin=%+v padding=%+v", tall.cfg.Margin, tall.cfg.Padding)
	}

	ungated := newRenderer(Config{Margin: Spacing{Bottom: 10}}, 80, 5)
	if ungated.cfg.Margin.Bottom != 10 {
		t.Fatalf("MinHeight=0 must never collapse: %+v", ungated.cfg.Margin)
	}
}
