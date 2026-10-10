package main

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestPlacementNamesAndCycle(t *testing.T) {
	placements := []placement{placementFloating, placementLeft, placementRight, placementTop, placementBottom}
	names := []string{"floating", "left", "right", "top", "bottom"}
	for i, where := range placements {
		if where != placement(i) {
			t.Fatalf("%s value = %d, want %d", names[i], where, i)
		}
		if got := where.String(); got != names[i] {
			t.Fatalf("placement %d name = %q, want %q", i, got, names[i])
		}
		if got, want := where.next(), placements[(i+1)%len(placements)]; got != want {
			t.Fatalf("%s.next() = %s, want %s", where, got, want)
		}
		if got, want := where.prev(), placements[(i+len(placements)-1)%len(placements)]; got != want {
			t.Fatalf("%s.prev() = %s, want %s", where, got, want)
		}
		if got := where.next().prev(); got != where {
			t.Fatalf("%s.next().prev() = %s, want %s", where, got, where)
		}
		if got := where.prev().next(); got != where {
			t.Fatalf("%s.prev().next() = %s, want %s", where, got, where)
		}
	}
	for value := len(placements); value <= 255; value++ {
		if got := placement(value).String(); got != "floating" {
			t.Fatalf("invalid placement %d name = %q, want floating", value, got)
		}
		if got := placement(value).next(); got != placementFloating {
			t.Fatalf("invalid placement %d.next() = %s, want floating", value, got)
		}
		if got := placement(value).prev(); got != placementFloating {
			t.Fatalf("invalid placement %d.prev() = %s, want floating", value, got)
		}
	}
}

func TestComputeLayoutGeometry(t *testing.T) {
	for _, where := range []placement{placementFloating, placementLeft, placementRight, placementTop, placementBottom} {
		for _, panelOpen := range []bool{false, true} {
			name := where.String() + "/closed"
			if panelOpen {
				name = where.String() + "/open"
			}
			t.Run(name, func(t *testing.T) {
				for cols := 1; cols <= 100; cols++ {
					for rows := 1; rows <= 40; rows++ {
						layout := computeLayout(cols, rows, where, panelOpen)
						if again := computeLayout(cols, rows, where, panelOpen); again != layout {
							t.Fatalf("%dx%d: nondeterministic layout: %+v then %+v", cols, rows, layout, again)
						}
						contentRows := max(rows-1, 1)
						usable := uv.Rect(0, 0, cols, contentRows)
						footerY := -1
						if rows > 1 {
							footerY = rows - 1
						}
						if layout.FooterY != footerY {
							t.Fatalf("%dx%d: footer = %d, want %d", cols, rows, layout.FooterY, footerY)
						}
						if layout.Shell.Dx() < 1 || layout.Shell.Dy() < 1 {
							t.Fatalf("%dx%d: shell has no usable cell: %+v", cols, rows, layout)
						}
						for _, rect := range []uv.Rectangle{layout.Shell, layout.Panel, layout.Body} {
							if rect.Empty() {
								if rect != (uv.Rectangle{}) {
									t.Fatalf("%dx%d: empty rectangle is not zero: %v", cols, rows, rect)
								}
								continue
							}
							if !rect.In(usable) {
								t.Fatalf("%dx%d: rectangle %v outside usable area %v", cols, rows, rect, usable)
							}
							if footerY >= 0 && rect.Max.Y > footerY {
								t.Fatalf("%dx%d: rectangle %v overlaps footer row %d", cols, rows, rect, footerY)
							}
						}

						visible := panelOpen && cols >= 3 && contentRows >= 3
						switch where {
						case placementLeft, placementRight:
							visible = visible && cols >= 4
						case placementTop, placementBottom:
							visible = visible && contentRows >= 4
						}
						if !visible {
							if layout.Panel != (uv.Rectangle{}) || layout.Body != (uv.Rectangle{}) || layout.Shell != usable {
								t.Fatalf("%dx%d: closed/suspended pane must leave the whole shell: %+v", cols, rows, layout)
							}
							continue
						}
						if layout.Panel.Dx() < 3 || layout.Panel.Dy() < 3 {
							t.Fatalf("%dx%d: visible pane is below the minimum size: %+v", cols, rows, layout)
						}
						if layout.Body != layout.Panel {
							t.Fatalf("%dx%d: body = %v, want the full borderless panel %v", cols, rows, layout.Body, layout.Panel)
						}

						switch where {
						case placementFloating:
							if layout.Shell != usable || !layout.Panel.In(layout.Shell) {
								t.Fatalf("%dx%d: floating pane must overlay the full shell: %+v", cols, rows, layout)
							}
							if layout.Panel.Min.X != (cols-layout.Panel.Dx())/2 || layout.Panel.Min.Y != (contentRows-layout.Panel.Dy())/2 {
								t.Fatalf("%dx%d: floating pane is not centered: %+v", cols, rows, layout)
							}
							if layout.Panel.Dx() > 100 || layout.Panel.Dy() > 30 {
								t.Fatalf("%dx%d: floating pane exceeds 100x30 cap: %+v", cols, rows, layout)
							}
							if cols >= 5 && (layout.Panel.Min.X < 1 || layout.Panel.Max.X >= cols) {
								t.Fatalf("%dx%d: floating pane lost available horizontal margins: %+v", cols, rows, layout)
							}
							if contentRows >= 5 && (layout.Panel.Min.Y < 1 || layout.Panel.Max.Y >= contentRows) {
								t.Fatalf("%dx%d: floating pane lost available vertical margins: %+v", cols, rows, layout)
							}
							if cols >= 10 && (layout.Panel.Dx()*10 < cols*7 || layout.Panel.Dx()*10 > cols*9) {
								t.Fatalf("%dx%d: floating width is not roughly 80%%: %+v", cols, rows, layout)
							}
							if contentRows >= 10 && (layout.Panel.Dy()*10 < contentRows*7 || layout.Panel.Dy()*10 > contentRows*9) {
								t.Fatalf("%dx%d: floating height is not roughly 80%%: %+v", cols, rows, layout)
							}
						case placementLeft, placementRight, placementTop, placementBottom:
							if layout.Panel.Overlaps(layout.Shell) || layout.Panel.Dx()*layout.Panel.Dy()+layout.Shell.Dx()*layout.Shell.Dy() != cols*contentRows {
								t.Fatalf("%dx%d: split panes do not tile usable area: %+v", cols, rows, layout)
							}
							if where == placementLeft || where == placementRight {
								width := max(3, cols/2)
								panel := uv.Rect(0, 0, width, contentRows)
								shell := uv.Rect(width, 0, cols-width, contentRows)
								if where == placementRight {
									panel = uv.Rect(cols-width, 0, width, contentRows)
									shell = uv.Rect(0, 0, cols-width, contentRows)
								}
								if layout.Panel != panel || layout.Shell != shell {
									t.Fatalf("%dx%d: %s split = %+v, want panel %v and shell %v", cols, rows, where, layout, panel, shell)
								}
							} else {
								height := max(3, contentRows/2)
								panel := uv.Rect(0, 0, cols, height)
								shell := uv.Rect(0, height, cols, contentRows-height)
								if where == placementBottom {
									panel = uv.Rect(0, contentRows-height, cols, height)
									shell = uv.Rect(0, 0, cols, contentRows-height)
								}
								if layout.Panel != panel || layout.Shell != shell {
									t.Fatalf("%dx%d: %s split = %+v, want panel %v and shell %v", cols, rows, where, layout, panel, shell)
								}
							}
						}
					}
				}
			})
		}
	}
}

func TestComputeLayoutClampsOuterSize(t *testing.T) {
	for _, size := range [][2]int{{-10, -10}, {-10, 40}, {100, -10}, {0, 0}, {0, 8}, {8, 0}} {
		cols, rows := size[0], size[1]
		for _, where := range []placement{placementFloating, placementLeft, placementRight, placementTop, placementBottom} {
			for _, panelOpen := range []bool{false, true} {
				got := computeLayout(cols, rows, where, panelOpen)
				want := computeLayout(max(cols, 1), max(rows, 1), where, panelOpen)
				if got != want {
					t.Fatalf("%dx%d %s open=%t: layout = %+v, want clamped %+v", cols, rows, where, panelOpen, got, want)
				}
			}
		}
	}
}

func TestComputeLayoutInvalidPlacement(t *testing.T) {
	for value := int(placementBottom) + 1; value <= 255; value++ {
		for _, size := range [][2]int{{1, 1}, {2, 4}, {3, 4}, {4, 5}, {5, 6}, {7, 9}, {81, 25}, {240, 100}} {
			cols, rows := size[0], size[1]
			for _, panelOpen := range []bool{false, true} {
				got := computeLayout(cols, rows, placement(value), panelOpen)
				want := computeLayout(cols, rows, placementFloating, panelOpen)
				if got != want {
					t.Fatalf("%dx%d invalid placement %d open=%t: layout = %+v, want floating %+v", cols, rows, value, panelOpen, got, want)
				}
			}
		}
	}
}

func TestComputeLayoutFloatingCap(t *testing.T) {
	layout := computeLayout(240, 100, placementFloating, true)
	if layout.Shell != uv.Rect(0, 0, 240, 99) || layout.FooterY != 99 {
		t.Fatalf("large floating pane changed shell/footer geometry: %+v", layout)
	}
	if layout.Panel != uv.Rect(70, 34, 100, 30) || layout.Body != layout.Panel {
		t.Fatalf("large floating pane must be centered and capped at 100x30: %+v", layout)
	}
}
