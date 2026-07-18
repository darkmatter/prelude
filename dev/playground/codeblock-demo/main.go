// Throwaway demo: recipe codeblocks as numbered, framed panels.
//
// Style under evaluation (vs the current CodeblockTopRuleFade):
//
//   - dim section label above everything
//
//   - recipe title as a `# comment` line *outside* the frame
//
//   - thin square frame, one padding row top and bottom inside
//
//   - numbered steps: dim gutter number, accent `$` prompt, bright command
//
//     go run ./codeblock-demo
package main

import (
	"fmt"
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const width = 96 // panel width, in cells

// Palette lifted from the motd playground (phosphor-green over neutrals).
var (
	bg     = lipgloss.Color("#0b0f0d")
	fg     = lipgloss.Color("#cbd5cd")
	muted  = lipgloss.Color("#7e908a")
	dim    = lipgloss.Color("#4a5751")
	accent = lipgloss.Color("#5fd7a0")
)

var frameC = lipgloss.Lighten(bg, 0.12)

type recipe struct {
	title string
	steps []string
}

var recipes = []recipe{
	{"spin up a clean local stack", []string{
		"just db:up",
		"just db:migrate && just db:seed",
		"just dev",
	}},
	{"ship a hotfix to production", []string{
		"git checkout -b fix/login",
		"just test && just build",
		"just deploy",
	}},
}

func paint(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Background(bg).Inline(true)
}

// row fills a line to the full panel width on the page background.
func row(content string) string {
	pad := max(width-ansi.StringWidth(content), 0)
	return content + paint(bg).Render(strings.Repeat(" ", pad))
}

func frameTop() string {
	return row(paint(frameC).Render("┌" + strings.Repeat("─", width-2) + "┐"))
}
func frameBot() string {
	return row(paint(frameC).Render("└" + strings.Repeat("─", width-2) + "┘"))
}

// framed wraps content between the vertical bars of the frame.
func framed(content string) string {
	bar := paint(frameC).Render("│")
	pad := max(width-2-ansi.StringWidth(content), 0)
	return row(bar + content + paint(bg).Render(strings.Repeat(" ", pad)) + bar)
}

// step renders one numbered command line: dim number, accent $, bright command.
func step(n int, cmd string) string {
	return paint(dim).Render(fmt.Sprintf("  %2d ", n)) +
		paint(accent).Render("$ ") +
		paint(fg).Bold(true).Render(cmd)
}

func block(r recipe) []string {
	out := []string{
		row(paint(muted).Render("# " + r.title)),
		row(""),
		frameTop(),
		framed(""),
	}
	for i, cmd := range r.steps {
		out = append(out, framed(step(i+1, cmd)))
	}
	return append(out, framed(""), frameBot())
}

func main() {
	lines := []string{
		row(""),
		row(paint(dim).Render("recipes — common flows that take a few steps")),
		row(""),
	}
	for i, r := range recipes {
		if i > 0 {
			lines = append(lines, row(""))
		}
		lines = append(lines, block(r)...)
	}
	lines = append(lines, row(""))

	out := strings.Join(lines, "\n") + "\n"
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 && w < width {
		fmt.Fprintln(os.Stderr, "codeblock-demo: terminal narrower than", width, "cells; expect wrapping")
	}
	fmt.Print(out)
}
