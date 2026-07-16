package ui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestContextBlendBackgroundWeight(t *testing.T) {
	background := lipgloss.Color("#102030")
	tint := lipgloss.Color("#d0e0f0")
	context := NewContext(Palette{}, background, false)

	tests := []struct {
		name   string
		weight float64
		want   color.Color
	}{
		{name: "below zero clamps to background", weight: -1, want: background},
		{name: "zero returns background", weight: 0, want: background},
		{name: "one returns tint", weight: 1, want: tint},
		{name: "above one clamps to tint", weight: 2, want: tint},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertSameColor(t, context.BlendBackground(tint, test.weight), test.want)
		})
	}
}

func TestContextBlendBackgroundMixesBothColors(t *testing.T) {
	background := lipgloss.Color("#000000")
	tint := lipgloss.Color("#ffffff")
	context := NewContext(Palette{}, background, false)

	got := context.BlendBackground(tint, 0.5)
	if sameColor(got, background) || sameColor(got, tint) {
		t.Fatalf("midpoint blend = %v, want a color between background and tint", got)
	}
}

func TestContextBlendBackgroundHandlesMissingColors(t *testing.T) {
	background := lipgloss.Color("#102030")
	tint := lipgloss.Color("#d0e0f0")

	assertSameColor(t, NewContext(Palette{}, nil, false).BlendBackground(tint, 0.5), tint)
	assertSameColor(t, NewContext(Palette{}, background, false).BlendBackground(nil, 0.5), background)
}

func assertSameColor(t *testing.T, got, want color.Color) {
	t.Helper()
	if !sameColor(got, want) {
		t.Fatalf("color = %v, want %v", got, want)
	}
}

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
