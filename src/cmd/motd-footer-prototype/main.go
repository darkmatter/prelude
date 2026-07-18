// PROTOTYPE — throw this command away after choosing a footer treatment.
// Question: should the dogfood MOTD status footer use no chrome, a subtle fill, or a native Lip Gloss border?
package main

import (
	"fmt"
	imagecolor "image/color"
	"os"
	"strings"

	"prelude/pkg/shared"

	"charm.land/lipgloss/v2"
)

const (
	cardWidth    = 88
	paddingX     = 4
	contentWidth = cardWidth - 2*paddingX
)

// Exact `minted` palette selected by root prelude.nix.
var palette = shared.Palette{
	Bg:           "#0c0c13",
	Surface:      "#161623",
	Secondary:    "#24243f",
	Fg:           "#b1b1bf",
	Muted:        "#6a6c85",
	Dim:          "#4a5585",
	Border:       "#1d1d2f",
	AccentBorder: "#3e4441",
	Accent:       "#f2cdcd",
	Accent2:      "#CC99FF",
	Error:        "#ee848e",
	SelectionFg:  "#0c0c13",
}

func main() {
	variants := []string{
		variant("A — current / plain", plainFooter()),
		variant("B — subtle footer background", backgroundFooter()),
		variant("C — native Lip Gloss top border", borderedFooter()),
	}
	fmt.Fprintln(os.Stdout, lipgloss.NewStyle().Background(color(palette.Bg)).Render(strings.Join(variants, "\n\n")))
}

func variant(name, footer string) string {
	nameLine := lipgloss.NewStyle().Foreground(color(palette.Dim)).Render(name)
	body := lipgloss.NewStyle().
		Width(cardWidth).
		Padding(1, paddingX, 2).
		Foreground(color(palette.Fg)).
		Background(color(palette.Surface)).
		Render(strings.Join([]string{
			title(),
			lipgloss.NewStyle().Foreground(color(palette.Accent)).Render(strings.Repeat("━", contentWidth)),
			"",
			header(),
			"",
			description(),
			"",
			sectionTitle("commands"),
			"",
			command("menu", "open this command menu"),
			command("check", "build + render smoke tests"),
			command("previews", "build the render checks and show their output"),
			command("titles", "inspect rendered titles"),
			"",
			sectionTitle("examples"),
			"",
			recipe(),
			"",
			footer,
		}, "\n"))
	return nameLine + "\n" + body
}

func title() string {
	art := strings.Join([]string{
		"█▀▀▀▄ █▀▀▀▄ █▀▀▀▀ █     █   █ █▀▀▀▄ █▀▀▀▀",
		"█▀▀▀  █▀▀▀▄ █▀▀   █     █   █ █   █ █▀▀",
		"▀     ▀   ▀ ▀▀▀▀▀ ▀▀▀▀▀  ▀▀▀  ▀▀▀▀  ▀▀▀▀▀",
	}, "\n")
	return lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, lipgloss.NewStyle().Foreground(color(palette.Accent)).Render(art))
}

func header() string {
	tagline := lipgloss.NewStyle().Foreground(color(palette.Fg)).Render("Dev Shell Activated")
	hints := lipgloss.NewStyle().Foreground(color(palette.Dim)).Render("[?] motd   [m] menu   [d] docs")
	first := tagline + strings.Repeat(" ", max(contentWidth-lipgloss.Width(tagline)-lipgloss.Width(hints), 1)) + hints
	subtitle := lipgloss.NewStyle().Foreground(color(palette.Muted)).Render("Your environment is ready")
	return first + "\n" + subtitle
}

func description() string {
	return lipgloss.NewStyle().Foreground(color(palette.Muted)).Width(contentWidth).Render(
		"This shell pins every tool the repo needs — compilers, linters, and language servers are already on your PATH. No global installs, and your host machine stays untouched.",
	)
}

func sectionTitle(text string) string {
	return lipgloss.NewStyle().Foreground(color(palette.Accent2)).Render(text)
}

func command(name, description string) string {
	prefix := lipgloss.NewStyle().Foreground(color(palette.Accent)).Render("$ " + name + " ")
	desc := lipgloss.NewStyle().Foreground(color(palette.Muted)).Render(description)
	leaders := lipgloss.NewStyle().Foreground(color(palette.Dim)).Render(strings.Repeat("·", max(contentWidth-lipgloss.Width(prefix)-lipgloss.Width(desc)-1, 3)))
	return prefix + leaders + " " + desc
}

func recipe() string {
	quiet := lipgloss.NewStyle().Foreground(color(palette.Dim))
	commandStyle := lipgloss.NewStyle().Foreground(color(palette.Muted))
	return strings.Join([]string{
		quiet.Render("─ tour the feature demos ") + quiet.Render(strings.Repeat("─", contentWidth-25)),
		"  " + quiet.Render("# acme-web showcase, then every theme"),
		"  " + commandStyle.Render("nix run .#example-motd"),
		"  " + commandStyle.Render("nix run .#example-themes"),
		"  " + commandStyle.Render("nix run .#examples"),
		quiet.Render(strings.Repeat("─", contentWidth)),
	}, "\n")
}

func plainFooter() string {
	return footerContent(lipgloss.NewStyle())
}

func backgroundFooter() string {
	return footerContent(lipgloss.NewStyle().
		Background(color(palette.Secondary)).
		Padding(1, 2))
}

func borderedFooter() string {
	return footerContent(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true, true, true, true).
		BorderForeground(lipgloss.Alpha(color(palette.AccentBorder), 0.2)).
		Background(lipgloss.Alpha(color(palette.Surface), 0.2)).
		PaddingTop(1))
}

func footerContent(chrome lipgloss.Style) string {
	label := lipgloss.NewStyle().Foreground(color(palette.Muted))
	dot := lipgloss.NewStyle().Foreground(color(palette.Accent))
	hint := lipgloss.NewStyle().Foreground(color(palette.Dim))
	status := label.Render("dev server  ") + dot.Render("●") + label.Render("  ·  flake  ") + dot.Render("●")
	status = lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, status)
	help := lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, hint.Render("last checked 6m ago · reload shell for latest status"))
	return chrome.Width(contentWidth).Render(status + "\n\n" + help)
}

func color(value shared.Color) imagecolor.Color {
	return lipgloss.Color(string(value))
}
