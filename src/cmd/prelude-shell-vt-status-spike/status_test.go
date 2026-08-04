package main

import (
	"image/color"
	"os/exec"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestComputeLayoutReservesTwoStatusRows(t *testing.T) {
	layout := computeLayout(80, 24)
	if layout.shellRows != 22 || layout.statusTop != 22 || layout.statusRows != 2 {
		t.Fatalf("computeLayout(80, 24) = %+v, want 22 shell rows followed by 2 status rows", layout)
	}
}

func TestRgbaTo256MapsExtremes(t *testing.T) {
	black := rgbaTo256(color.RGBA{})
	white := rgbaTo256(color.RGBA{R: 255, G: 255, B: 255, A: 255})
	if black != 16 {
		t.Fatalf("rgbaTo256(black) = %d, want 16", black)
	}
	if white != 231 {
		t.Fatalf("rgbaTo256(white) = %d, want 231", white)
	}
}

func TestShellSingleRoundtripsThroughBash(t *testing.T) {
	for _, value := range []string{"simple", "it's here", "with \"quotes\" and 'apos'"} {
		quoted := shellSingle(value)
		out, err := exec.Command("bash", "-c", "printf '%s' "+quoted).CombinedOutput()
		if err != nil {
			t.Fatalf("bash rejected %q -> %s: %s (%s)", value, quoted, out, err)
		}
		if string(out) != value {
			t.Fatalf("roundtrip %q through bash = %q, want %q (quoted form %s)", value, out, value, quoted)
		}
	}
}

func TestCatalogFragmentEmitsDataAndPaletteIndices(t *testing.T) {
	catalog := menuCatalog{
		Palette: menuPalette{Accent: "#cc99ff", Muted: "#a1a1aa"},
		Groups: []menuGroup{{
			Title: "build",
			Tasks: []menuCommand{{
				Name:        "build",
				Command:     "build",
				Description: "build a flake output",
				Args:        []menuArg{{Token: "<target>", Options: []string{".#motd", ".#menu"}}},
			}},
		}},
	}
	for _, task := range catalog.Groups[0].Tasks {
		catalog.commands = append(catalog.commands, task)
	}
	fragment := string(catalog.emitShellCatalog())

	for _, want := range []string{
		"prelude_palette_accent=",
		"prelude_palette_muted=",
		"prelude_catalog_names=( 'build' )",
		"prelude_catalog_commands=( 'build' )",
		"prelude_catalog_descriptions=( 'build a flake output' )",
		"prelude_catalog_argopts=( '.#motd .#menu' )",
		"prelude_direct_commands=( 'build' )",
	} {
		if !strings.Contains(fragment, want) {
			t.Fatalf("catalog fragment missing %q\n%s", want, fragment)
		}
	}
}

func TestEmbeddedBleScriptRegistersCompletions(t *testing.T) {
	for _, want := range []string{
		"complete -F prelude/complete/x x",
		"complete -F prelude/complete/direct",
		"compopt -o nosort 2>/dev/null || true",
		"bleopt complete_menu_style=desc",
		"bleopt complete_menu_filter=1",
		"bleopt complete_auto_complete=1",
		"bleopt edit_magic_accept=sabbrev:verify-syntax",
		"bleopt info_display=top",
		"blehook/eval-after-load complete prelude/ble/setup",
	} {
		if !strings.Contains(preludeBleScript, want) {
			t.Fatalf("embedded prelude-ble.sh missing %q", want)
		}
	}
}

func TestIdleStatusShowsMotdLifecycle(t *testing.T) {
	catalog := menuCatalog{
		MOTDCommands: []commandHint{{Command: "build"}, {Command: "test"}},
		commands:     []menuCommand{{Name: "build", Command: "build"}},
	}
	model := &hostModel{
		state:   hostState{layout: computeLayout(80, 24)},
		catalog: catalog,
	}
	frame := uv.NewScreenBuffer(80, 24)
	model.drawStatus(&frame)

	primary := screenRowText(frame, 22, 80)
	if !strings.Contains(primary, "build") || !strings.Contains(primary, "test") {
		t.Fatalf("idle row does not show MOTD lifecycle: %q", primary)
	}
	footer := strings.TrimSpace(screenRowText(frame, 23, 80))
	if !strings.HasSuffix(footer, "● ready") {
		t.Fatalf("idle footer not ready: %q", footer)
	}
}

func TestUnavailableStatusWhenCatalogueEmpty(t *testing.T) {
	model := &hostModel{
		state:   hostState{layout: computeLayout(80, 24)},
		catalog: menuCatalog{},
	}
	frame := uv.NewScreenBuffer(80, 24)
	model.drawStatus(&frame)
	footer := strings.TrimSpace(screenRowText(frame, 23, 80))
	if !strings.HasSuffix(footer, "● unavailable") {
		t.Fatalf("empty-catalogue footer = %q, want ● unavailable", footer)
	}
}

func screenRowText(frame uv.ScreenBuffer, y, width int) string {
	var row strings.Builder
	for x := 0; x < width; x++ {
		cell := frame.CellAt(x, y)
		if cell == nil || cell.Content == "" {
			row.WriteByte(' ')
			continue
		}
		row.WriteString(cell.Content)
	}
	return row.String()
}
