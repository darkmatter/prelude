package motd

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
)

func TestNewStylesBlendsCodeBackgroundWithPaletteSurface(t *testing.T) {
	model := PaintModel{
		Background: "#000000",
		Config: Config{Palette: shared.Palette{
			Bg:      "#000000",
			Surface: "#ffffff",
		}},
	}

	got := newStyles(model).codeBg
	want := lipgloss.Blend1D(101, lipgloss.Color(model.Background), lipgloss.Color(model.Config.Palette.Surface.String()))[50]
	if !colorsEqual(got, want) {
		t.Fatalf("code background = %v, want weighted surface blend %v", got, want)
	}
}

func colorsEqual(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
