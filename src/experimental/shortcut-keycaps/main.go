// Command shortcut-keycaps is a throwaway visual comparison for MOTD shortcut
// treatments. Run with: go run ./experimental/shortcut-keycaps
package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
	"prelude/pkg/ui"
)

const renderWidth = 72

func main() {
	palette := ui.Palette{
		Bg:           shared.Color("#0c110e"),
		Surface:      shared.Color("#131715"),
		Secondary:    shared.Color("#202622"),
		Fg:           shared.Color("#d5e2d7"),
		Muted:        shared.Color("#7d8a81"),
		Dim:          shared.Color("#5d665f"),
		Border:       shared.Color("#1a201d"),
		AccentBorder: shared.Color("#284a2c"),
		Accent:       shared.Color("#68e371"),
		Accent2:      shared.Color("#d7be72"),
		Error:        shared.Color("#dc5b66"),
		SelectionFg:  shared.Color("#0c110e"),
	}
	ctx := ui.NewContext(palette, lipgloss.Color(palette.Bg.String()), false)

	headline(ctx, "shortcut keycap explorations")
	fmt.Println(ctx.Muted().Render("Throwaway prototype — every treatment is shown in the MOTD activation layout."))

	label := ctx.Muted().Bold(true)
	key := lipgloss.NewStyle().
		Foreground(ctx.Background).
		Background(ctx.Color(ctx.Palette.Accent)).
		Bold(true).
		Padding(0, 1)
	secondaryLabel := ctx.Foreground().
		Background(ctx.Color(ctx.Palette.Secondary)).
		Padding(0, 1)
	grayLabel := ctx.Muted().
		Background(ctx.Color(ctx.Palette.Secondary)).
		Padding(0, 1)
	grayKey := ctx.Accent().
		Background(ctx.Color(ctx.Palette.Secondary)).
		Bold(true).
		Padding(0, 1)
	outline := ctx.Foreground().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder(), false, true, false, true).
		BorderForeground(ctx.Color(ctx.Palette.AccentBorder))

	variant(ctx, "current filled key", key.Render("m")+ctx.Fill().Render(" ")+label.Render("menu"))
	variant(ctx, "two-tone chip", key.Render("m")+secondaryLabel.Render("menu"))
	variant(ctx, "two-tone with gap", key.Render("m")+ctx.Fill().Render(" ")+secondaryLabel.Render("menu"))
	variant(ctx, "outlined one-row", outline.Render("m")+ctx.Fill().Render(" ")+label.Render("menu"))
	variant(ctx, "bracketed", ctx.Dim().Render("[")+ctx.Accent().Bold(true).Render("m")+ctx.Dim().Render("]  ")+label.Render("menu"))
	variant(ctx, "reverse text", ctx.Accent().Reverse(true).Bold(true).Render(" m ")+ctx.Fill().Render(" ")+ctx.Foreground().Bold(true).Render("menu"))
	variant(ctx, "accent underline", ctx.Accent().Bold(true).Underline(true).Render("m")+ctx.Fill().Render(" ")+label.Render("menu"))

	fmt.Println()
	fmt.Println(ctx.Dim().Render(strings.Repeat("─", renderWidth)))
	fmt.Println(ctx.Foreground().Bold(true).Render("true rounded border — real border, three terminal rows"))
	fmt.Println()
	fullKey := lipgloss.NewStyle().
		Foreground(ctx.Background).
		Background(ctx.Color(ctx.Palette.Accent)).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ctx.Color(ctx.Palette.AccentBorder)).
		Render("m")
	fmt.Println(ctx.Dim().Render(strings.Repeat("━", renderWidth)))
	fmt.Println(ui.PlaceRight(renderWidth, ctx.Accent2().Bold(true).Render("Dev Shell Activated"), fullKey, ctx.Fill()))
	fmt.Println(ctx.Muted().Faint(true).Render("Your environment is ready"))

	fmt.Println()
	fmt.Println(ctx.Dim().Render(strings.Repeat("─", renderWidth)))
	fmt.Println(ctx.Foreground().Bold(true).Render("three-command treatment gallery"))
	fmt.Println()

	rails := ctx.Foreground().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder(), false, true, false, true).
		BorderForeground(ctx.Color(ctx.Palette.AccentBorder))
	bracket := ctx.Accent().Bold(true)
	prompt := ctx.Accent2().Bold(true)
	dot := ctx.Accent().Bold(true)
	underline := ctx.Accent().Bold(true).Underline(true)
	plainLabel := ctx.Foreground().Bold(true)
	quietLabel := ctx.Muted()
	warmKey := ctx.Accent2().Bold(true)
	surfaceKey := ctx.Foreground().Bold(true).
		Background(ctx.Color(ctx.Palette.Surface)).
		Padding(0, 1)
	surfaceLabel := ctx.Muted().
		Background(ctx.Color(ctx.Palette.Surface)).
		Padding(0, 1)

	galleryRow(ctx, "quiet brackets", bracketRow(ctx, bracket, quietLabel))
	galleryRow(ctx, "warm brackets", bracketRow(ctx, warmKey, quietLabel))
	galleryRow(ctx, "surface chip", shortcutRow(ctx, surfaceKey, surfaceLabel, ""))
	galleryRow(ctx, "shared gray chip", shortcutRow(ctx, grayKey, grayLabel, ""))
	galleryRow(ctx, "two-tone joined", shortcutRow(ctx, key, secondaryLabel, ""))
	galleryRow(ctx, "one-row rails", shortcutRow(ctx, rails, plainLabel, " "))
	galleryRow(ctx, "prompt markers", promptRow(ctx, prompt, quietLabel))
	galleryRow(ctx, "status dots", dotRow(ctx, dot, secondaryLabel))
	galleryRow(ctx, "minimal underline", shortcutRow(ctx, underline, quietLabel, " "))
	galleryRow(ctx, "spaced two-tone", shortcutRow(ctx, key, secondaryLabel, "  "))

	fmt.Println()
	fmt.Println(ctx.Dim().Render(strings.Repeat("─", renderWidth)))
	fmt.Println(ctx.Foreground().Bold(true).Render("recommended row — quiet bracketed keys"))
	fmt.Println()
	row := bracketRow(ctx, bracket, quietLabel)
	variant(ctx, "recommended bracketed row", row)
}

func headline(ctx ui.Context, text string) {
	fmt.Println(ctx.Accent().Bold(true).Render(text))
	fmt.Println(ctx.Dim().Render(strings.Repeat("━", renderWidth)))
}

func shortcutRow(ctx ui.Context, key, label lipgloss.Style, gap string) string {
	item := func(shortcut, command string) string {
		return key.Render(shortcut) + ctx.Fill().Render(gap) + label.Render(command)
	}
	separator := ctx.Dim().Render("  ·  ")
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		item("?", "help"), separator,
		item("m", "menu"), separator,
		item("d", "docs"),
	)
}

func bracketRow(ctx ui.Context, key, label lipgloss.Style) string {
	item := func(shortcut, command string) string {
		return ctx.Dim().Render("[") + key.Render(shortcut) + ctx.Dim().Render("] ") + label.Render(command)
	}
	separator := ctx.Dim().Render("   ")
	return lipgloss.JoinHorizontal(lipgloss.Top, item("?", "help"), separator, item("m", "menu"), separator, item("d", "docs"))
}

func promptRow(ctx ui.Context, prompt, label lipgloss.Style) string {
	item := func(shortcut, command string) string {
		return prompt.Render(shortcut+"›") + ctx.Fill().Render(" ") + label.Render(command)
	}
	separator := ctx.Dim().Render("  /  ")
	return lipgloss.JoinHorizontal(lipgloss.Top, item("?", "help"), separator, item("m", "menu"), separator, item("d", "docs"))
}

func dotRow(ctx ui.Context, dot, label lipgloss.Style) string {
	item := func(shortcut, command string) string {
		return dot.Render("●") + ctx.Fill().Render(" ") + ctx.Accent().Bold(true).Render(shortcut) + ctx.Fill().Render(" ") + label.Render(command)
	}
	separator := ctx.Dim().Render("   ")
	return lipgloss.JoinHorizontal(lipgloss.Top, item("?", "help"), separator, item("m", "menu"), separator, item("d", "docs"))
}

func galleryRow(ctx ui.Context, label, sample string) {
	fmt.Println(ctx.Dim().Bold(true).Width(19).Render(label) + sample)
}

func variant(ctx ui.Context, name, shortcut string) {
	fmt.Println()
	fmt.Println(ctx.Dim().Render(name))
	fmt.Println(ctx.Dim().Render(strings.Repeat("━", renderWidth)))
	fmt.Println(ui.PlaceRight(
		renderWidth,
		ctx.Accent2().Bold(true).Render("Dev Shell Activated"),
		shortcut,
		ctx.Fill(),
	))
	fmt.Println(ctx.Muted().Faint(true).Render("Your environment is ready"))
}
