package main

import "testing"

func TestComputeLayoutPinOff(t *testing.T) {
	g := computeLayout(80, 40, 10, pinOff)
	if g.pinH != 0 || g.shellH != 39 || g.shellTop != 1 || g.statusRow != 40 {
		t.Fatalf("pinOff: %+v", g)
	}
}

func TestComputeLayoutPinOnShell10(t *testing.T) {
	g := computeLayout(80, 40, 10, pinMotd)
	// status 1 + shell 10 + pin 29 = 40
	if g.shellH != 10 || g.pinH != 29 || g.shellTop != 30 || g.statusRow != 40 {
		t.Fatalf("pinOn10: %+v", g)
	}
}

func TestComputeLayoutPinOnShell1(t *testing.T) {
	g := computeLayout(80, 40, 1, pinDocs)
	if g.shellH != 1 || g.pinH != 38 || g.shellTop != 39 {
		t.Fatalf("pinOn1: %+v", g)
	}
}

func TestPinCycle(t *testing.T) {
	m := pinOff
	for _, want := range []pinMode{pinMotd, pinMenu, pinDocs, pinOff} {
		m = m.next()
		if m != want {
			t.Fatalf("got %v want %v", m, want)
		}
	}
}
