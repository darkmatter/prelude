// MOTD renderer for the `prelude` devshell.
//
// Styling is done entirely with charm.land/lipgloss/v2. Everything intended to
// be tweaked lives in the CONFIG block below; the rendering code beneath it
// should rarely need to change.
//
//	go run .
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

// ═══════════════════════════════ CONFIG ═════════════════════════════════════
// Edit these — the next agent should be able to reskin/retheme from here alone.

// ── Layout ───────────────────────────────────────────────────────────────────
const cardWidth = 60 // inner width of the MOTD card, in terminal cells

// ── Palette (phosphor-green terminal theme) ───────────────────────────────────
// A single green accent over neutrals; amber is reserved for git / warnings.
var (
	bg     = lipgloss.Color("#0b0f0d") // near-black background
	fg     = lipgloss.Color("#cbd5cd") // primary text
	muted  = lipgloss.Color("#7e908a") // secondary text
	dim    = lipgloss.Color("#4a5751") // labels, gutters, captions
	accent = lipgloss.Color("#5fd7a0") // phosphor green (primary highlight)
	amber  = lipgloss.Color("#e0b877") // git branch / warnings
)

// Derived surfaces (see lipgloss Lighten / Darken).
var (
	headerBg = lipgloss.Lighten(bg, 0.05) // subtly lifted hero bar
	panelBg  = lipgloss.Darken(bg, 0.30)  // recessed recipe "well"
)

// dividerPeak is the (dimmed) accent that section dividers glow toward.
var dividerPeak = lipgloss.Darken(accent, 0.35)

// ── Title style ───────────────────────────────────────────────────────────────
// The wordmark treatment. Switch between the two options here.
type TitleStyle int

const (
	TitleInverted  TitleStyle = iota // solid accent chip with bg-colored text
	TitleBracketed                   // [ name ] framed by accent brackets
)

const titleStyle = TitleInverted

// ── Unified header title ───────────────────────────────────────────────────────
// When the header has a background, the wordmark should share that same surface
// rather than introducing a second, brighter rectangle.
type HeaderTitleStyle int

const (
	HeaderTitlePlain    HeaderTitleStyle = iota // bold accent wordmark
	HeaderTitleSpine                            // accent spine + bright wordmark
	HeaderTitleBracketed                        // dim brackets + accent wordmark
	HeaderTitleLabel                            // small dim label + bright wordmark
)

const headerTitleStyle = HeaderTitleSpine

// ── Section heading style ──────────────────────────────────────────────────────
// How section headings ("commands", "examples") relate to their divider.
// Switch between the options here; main() currently renders all three to compare.
type HeadingStyle int

const (
	HeadingSpine   HeadingStyle = iota // a divider, then "▌ heading" on its own line
	HeadingInlineC                     // heading breaks the divider, centered
	HeadingInlineL                     // heading breaks the divider, near the left
)

const headingStyle = HeadingSpine

// ── Commands section style ─────────────────────────────────────────────────────
// How each entry in the "commands" list is rendered.
type CommandsStyle int

const (
	CommandsList           CommandsStyle = iota // `$ cmd` + column-aligned description
	CommandsLeaders                             // dotted leaders to a right-aligned description
	CommandsLeadersAligned                      // dotted leaders to a column-aligned description
)

const commandsStyle = CommandsLeaders

// ── Codeblock style ────────────────────────────────────────────────────────────
// How the examples codeblock is rendered. All variants are accent-free: the
// block chrome uses only neutrals so the commands inside stay the focus.
type CodeblockStyle int

const (
	CodeblockWell        CodeblockStyle = iota // flat recessed fill, title on the same surface
	CodeblockFramed                            // thin square frame, title inline in top edge
	CodeblockTopRule                           // top+bottom rule, title inline in the top rule
	CodeblockTopRuleFade                       // like TopRule, but the rules fade toward bg on the right
)

const codeblockStyle = CodeblockTopRuleFade

// ── Getting-started layout ─────────────────────────────────────────────────────
// How the combined commands + examples region is framed. Consolidating both
// under one heading cuts down on the number of horizontal rules on screen.
type StartLayout int

const (
	StartSubLabels StartLayout = iota // centered inline heading + dim sub-labels per group
	StartNoHeading                    // plain divider (no inline heading) + sub-labels
	StartHeadingBelow                 // plain divider, then centered heading line + sub-labels
)

const startLayout = StartHeadingBelow

// ── Content ───────────────────────────────────────────────────────────────────
const (
	projectName     = "prelude"
	tagline         = "everything you need to build, test & ship"
	statusLabel        = "nix develop  ·  flake @ 1a2b3c4"
	statusLabelCompact = "flake @ 1a2b3c4"
	statusText         = "ready"
	startHeading    = "Getting Started"
	commandsHeading = "commands"
	examplesHeading = "examples"
	gitBranch = "main"
	gitStatus = " ↑2 ●1"

	introText = "This shell pins every tool the repo needs — compilers, linters, and " +
		"language servers are already on your PATH. No global installs, and " +
		"your host machine stays untouched."
)

// Environment versions shown in the banner sub-row.
var envTools = []struct{ label, value string }{
	{"node", "22.3.0"},
	{"pnpm", "9.4.0"},
}

// The "commands" list.
var nextSteps = []struct{ cmd, desc string }{
	{"menu", "browse all project commands"},
	{"just dev", "start local development"},
	{"just test", "run the test suite"},
}

// A recipe step is either a command (cmd set) or a comment caption (comment set).
type recipeStep struct {
	cmd     string
	comment string
}

// A recipe is one titled codeblock in the "examples" section.
type recipe struct {
	heading string
	steps   []recipeStep
}

// The "examples" codeblocks, rendered in order.
var recipeList = []recipe{
	{
		heading: "spin up a clean local stack",
		steps: []recipeStep{
			{comment: "start postgres + redis first"},
			{cmd: "just db:up"},
			{cmd: "just db:migrate && just db:seed"},
			{comment: "now the app can boot"},
			{cmd: "just dev"},
		},
	},
	{
		heading: "reset & reseed the database",
		steps: []recipeStep{
			{comment: "drop, recreate, and re-migrate"},
			{cmd: "just db:reset"},
			{cmd: "just db:seed --demo"},
		},
	},
}

// ═════════════════════════════��═ RENDER ════════════════════════════════════���

func main() {
	termW := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termW = w
	}

	label := func(s string) string {
		return fillLine(inline(on(bg, dim)).Render("── "+s+" "), cardWidth, bg)
	}
	// Compare the current filled header bar with the older transparent header and
	// primary gradient underline. Both keep the footerless shortcut ending.
	full := func(name, hero string) string {
		return join(
			label(name),
			blank(),
			hero,
			blank(),
			description(),
			blank(),
			envRow(),
			blank(),
			gettingStarted(startLayout, codeblockStyle),
			blank(),
			shortcutLine(),
		)
	}
	body := join(
		full("header · filled bar", bannerStyled(HeaderTitleSpine)),
		blank(), blank(), blank(), blank(),
		full("header · transparent + gradient divider", bannerTransparent()),
	)

	fmt.Print(place(body, termW))
}

// ── sections ───────────�����──────────────────────────────────────────────────────

// banner is the hero: a left-aligned wordmark with a right-aligned status
// column on the same row, a pronounced glow underline, then the tagline.
func banner() string { return bannerStyled(headerTitleStyle) }

// bannerStyled renders the filled hero banner. The wordmark shares the bar's
// exact surface, avoiding a nested title background.
func bannerStyled(hts HeaderTitleStyle) string {
	surface := headerBg
	title := headerTitle(surface, hts)
	// Keep the right-hand status intact. Wider wordmarks switch to a compact
	// environment label instead of forcing the row past cardWidth.
	status := func(label string) string {
		return inline(on(surface, dim)).Render(label+"  ") +
			inline(on(surface, accent)).Render("● ") +
			inline(on(surface, muted)).Render(statusText)
	}
	// Reserve two columns at the right edge so the status never touches the
	// boundary of the header surface.
	const rightPad = 2
	contentWidth := cardWidth - rightPad
	info := status(statusLabel)
	if lipgloss.Width(title)+2+lipgloss.Width(info) > contentWidth {
		info = status(statusLabelCompact)
	}

	gap := max(contentWidth-lipgloss.Width(title)-lipgloss.Width(info), 1)
	row := fillLine(
		title+
			fill(surface).Render(strings.Repeat(" ", gap))+
			info+
			fill(surface).Render(strings.Repeat(" ", rightPad)),
		cardWidth,
		surface,
	)
	// The tagline always sits on the page background below the header.
	taglineRow := fillLine(inline(on(bg, dim)).Render(tagline), cardWidth, bg)

	// A compact filled header bar: the wordmark + status on one row, padded
	// above and below. The tagline sits independently on the page background.
	pad := fill(surface).Width(cardWidth).Render("")
	return join(pad, row, pad, blank(), taglineRow)
}

// bannerTransparent restores the older hero treatment: no enclosing background,
// the inverted title chip on the page surface, and a full-width gradient divider.
func bannerTransparent() string {
	surface := bg
	title := renderTitle(surface)
	status := func(label string) string {
		return inline(on(surface, dim)).Render(label+"  ") +
			inline(on(surface, accent)).Render("● ") +
			inline(on(surface, muted)).Render(statusText)
	}

	const rightPad = 2
	contentWidth := cardWidth - rightPad
	info := status(statusLabel)
	if lipgloss.Width(title)+2+lipgloss.Width(info) > contentWidth {
		info = status(statusLabelCompact)
	}
	gap := max(contentWidth-lipgloss.Width(title)-lipgloss.Width(info), 1)
	row := fillLine(
		title+
			fill(surface).Render(strings.Repeat(" ", gap))+
			info+
			fill(surface).Render(strings.Repeat(" ", rightPad)),
		cardWidth,
		surface,
	)
	taglineRow := fillLine(inline(on(bg, dim)).Render(tagline), cardWidth, bg)
	return join(row, blank(), glowRule("━", accent), blank(), taglineRow)
}

// renderTitle dispatches on the configured TitleStyle.
func renderTitle(surface color.Color) string {
	switch titleStyle {
	case TitleBracketed:
		return titleBracketed(surface)
	default:
		return titleInverted(surface)
	}
}

// titleInverted: a solid accent chip with surface-colored text — heaviest.
func titleInverted(surface color.Color) string {
	return inline(lipgloss.NewStyle().Foreground(surface).Background(accent).Bold(true)).
		Render("  " + projectName + "  ")
}

// titleBracketed: accent brackets frame a bright title — subtle, terminal feel.
func titleBracketed(surface color.Color) string {
	return inline(on(surface, dim)).Render("[ ") +
		inline(on(surface, accent).Bold(true)).Render(projectName) +
		inline(on(surface, dim)).Render(" ]")
}

// headerTitle renders a wordmark whose background is exactly the header surface.
// The variants differ only in typography/punctuation, never in fill color.
func headerTitle(surface color.Color, style HeaderTitleStyle) string {
	switch style {
	case HeaderTitlePlain:
		return inline(on(surface, accent).Bold(true)).Render("  " + projectName + "  ")
	case HeaderTitleBracketed:
		return inline(on(surface, dim)).Render("  [ ") +
			inline(on(surface, accent).Bold(true)).Render(projectName) +
			inline(on(surface, dim)).Render(" ]  ")
	case HeaderTitleLabel:
		return inline(on(surface, dim)).Render("  devshell / ") +
			inline(on(surface, fg).Bold(true)).Render(projectName) +
			inline(on(surface, dim)).Render("  ")
	default: // HeaderTitleSpine
		return inline(on(surface, accent)).Render("  ▌ ") +
			inline(on(surface, fg).Bold(true)).Render(projectName) +
			inline(on(surface, dim)).Render("  ")
	}
}

// description: onboarding prose, then the first two commands a newcomer runs.
func description() string {
	para := lipgloss.NewStyle().Foreground(fg).Background(bg).Width(cardWidth)

	tip1 := fill(bg).Width(cardWidth).Render(
		inline(on(bg, dim)).Render("first time here? run ") +
			inline(on(bg, accent).Bold(true)).Render("just setup") +
			inline(on(bg, muted)).Render(" to install git hooks and seed a local .env,"))
	tip2 := fill(bg).Width(cardWidth).Render(
		inline(on(bg, muted)).Render("then ") +
			inline(on(bg, accent).Bold(true)).Render("just dev") +
			inline(on(bg, muted)).Render(" to boot the full stack. Config lives in ") +
			inline(on(bg, fg)).Render("flake.nix") +
			inline(on(bg, muted)).Render("."))

	var out []string
	for _, l := range strings.Split(para.Render(introText), "\n") {
		out = append(out, fillLine(l, cardWidth, bg))
	}
	out = append(out, blank(), tip1, tip2)
	return join(out...)
}

// commands: the "commands" list, rendered with the configured CommandsStyle.
func commands(hs HeadingStyle) string { return commandsStyled(hs, commandsStyle) }

// commandsStyled renders the commands section with an explicit style.
func commandsStyled(hs HeadingStyle, cs CommandsStyle) string {
	maxCmd := 0
	for _, r := range nextSteps {
		if w := ansi.StringWidth(r.cmd); w > maxCmd {
			maxCmd = w
		}
	}

	parts := append(sectionOpen(commandsHeading, hs), blank())
	for _, row := range nextSteps {
		parts = append(parts, commandRow(row.cmd, row.desc, maxCmd, cs))
	}
	return join(parts...)
}

// commandRow renders one command entry according to the given CommandsStyle.
func commandRow(cmd, desc string, maxCmd int, cs CommandsStyle) string {
	switch cs {
	case CommandsLeadersAligned:
		// Dotted leaders bridge the command to the column-aligned description:
		// dots fill only the gap up to the shared description column.
		dots := maxCmd - ansi.StringWidth(cmd) + 2
		line := inline(on(bg, accent)).Render("$ ") +
			inline(on(bg, fg).Bold(true)).Render(cmd) +
			inline(on(bg, dim)).Render(" "+strings.Repeat("·", dots)+" ") +
			inline(on(bg, muted)).Render(desc)
		return fill(bg).Width(cardWidth).Render(line)
	case CommandsLeaders:
		// Dotted leaders bridge the command to a right-aligned description.
		left := inline(on(bg, accent)).Render("$ ") +
			inline(on(bg, fg).Bold(true)).Render(cmd)
		right := inline(on(bg, muted)).Render(desc)
		dots := max(cardWidth-2-ansi.StringWidth(cmd)-ansi.StringWidth(desc)-2, 1)
		leader := inline(on(bg, dim)).Render(" " + strings.Repeat("·", dots) + " ")
		return fill(bg).Width(cardWidth).Render(left + leader + right)
	default: // CommandsList
		// The baseline: `$ cmd` with the description column-aligned.
		gap := strings.Repeat(" ", maxCmd-ansi.StringWidth(cmd)+3)
		line := inline(on(bg, accent)).Render("$ ") +
			inline(on(bg, fg).Bold(true)).Render(cmd) +
			onBg(gap) +
			inline(on(bg, muted)).Render(desc)
		return fill(bg).Width(cardWidth).Render(line)
	}
}

// gettingStarted renders the combined commands + examples region under a single
// "Getting Started" heading, dispatching on the configured StartLayout. Folding
// both groups under one heading removes a divider and the old examples caption,
// leaving the top-rule blocks as the main horizontal lines in this region.
func gettingStarted(layout StartLayout, cbs CodeblockStyle) string {
	// subLabel is a quiet group caption, flush to the left edge.
	subLabel := func(text string) string {
		return fill(bg).Width(cardWidth).Render(
			inline(on(bg, dim)).Render(text),
		)
	}

	maxCmd := 0
	for _, r := range nextSteps {
		if w := ansi.StringWidth(r.cmd); w > maxCmd {
			maxCmd = w
		}
	}
	commandRows := func() []string {
		var out []string
		for _, row := range nextSteps {
			out = append(out, commandRow(row.cmd, row.desc, maxCmd, commandsStyle))
		}
		return out
	}
	exampleBlocks := func() []string {
		var out []string
		for i, r := range recipeList {
			if i > 0 {
				out = append(out, blank())
			}
			out = append(out, codeblock(r, cbs)...)
		}
		return out
	}

	// centeredHeading is the heading as a bold centered line of its own.
	centeredHeading := func(text string) string {
		return fill(bg).Width(cardWidth).Align(lipgloss.Center).Render(
			inline(on(bg, fg).Bold(true)).Render(text),
		)
	}

	// The sub-labelled body is shared: dim group captions before each group.
	labelledBody := func() []string {
		var out []string
		out = append(out, subLabel(commandsHeading), blank())
		out = append(out, commandRows()...)
		out = append(out, blank(), subLabel(examplesHeading), blank())
		out = append(out, exampleBlocks()...)
		return out
	}

	var body []string
	switch layout {
	case StartNoHeading:
		// A plain divider (no inline heading), then sub-labels do the work.
		body = append(body, divider(), blank())
		body = append(body, labelledBody()...)
	case StartHeadingBelow:
		// Plain divider, then a centered heading line above the sub-labels.
		body = append(body, divider(), blank(), centeredHeading(startHeading), blank())
		body = append(body, labelledBody()...)
	default: // StartSubLabels
		// Centered inline-divider heading, then dim sub-labels marking each group.
		body = append(body, headingRule(startHeading, -1), blank())
		body = append(body, labelledBody()...)
	}
	return join(body...)
}

// codeblock renders one recipe as a titled block. Chrome (edges, fills) is
// accent-free — neutrals only; the title is the sole accented element.
func codeblock(r recipe, cbs CodeblockStyle) []string {
	// stepLine renders one recipe line's content on the given surface.
	stepLine := func(s recipeStep, surface color.Color) string {
		if s.cmd == "" {
			return inline(on(surface, dim)).Render("# " + s.comment)
		}
		return inline(on(surface, fg).Bold(true)).Render(s.cmd)
	}

	switch cbs {
	case CodeblockFramed:
		// A thin square frame on plain bg — structure from the border alone.
		// Built manually (rather than splicing a lipgloss box) so every line is
		// exactly cardWidth wide. The border color is derived from bg (slightly
		// lightened) so it blends. Title sits inline in the top: ┌─ title ──┐
		frameC := lipgloss.Lighten(bg, 0.12)
		fc := inline(on(bg, frameC))
		vbar := fc.Render("│")

		title := " " + r.heading + " "
		trail := max(cardWidth-3-ansi.StringWidth(title), 0)
		top := fc.Render("┌─") +
			inline(on(bg, accent).Bold(true)).Render(title) +
			fc.Render(strings.Repeat("─", trail) + "┐")

		out := []string{top}
		for _, s := range r.steps {
			out = append(out, vbar+fillLine(" "+stepLine(s, bg), cardWidth-2, bg)+vbar)
		}
		return append(out, fc.Render("└"+strings.Repeat("─", cardWidth-2)+"┘"))

	case CodeblockTopRule, CodeblockTopRuleFade:
		// Horizontal rules top and bottom, title inline in the top: ─ title ──.
		// The whole block sits on a very slightly darkened surface (every line
		// filled to cardWidth so there is no left-edge seam), contents indented.
		blockBg := lipgloss.Darken(bg, 0.05)
		frameC := lipgloss.Lighten(blockBg, 0.12)
		fade := cbs == CodeblockTopRuleFade

		// ruleColor gives the dash color at a column: a flat frameC, or (in the
		// fade variant) a gradient from frameC on the left to blockBg on the
		// right so the rule dissolves into the background.
		var grad []color.Color
		if fade {
			// Fade toward — but not all the way to — the background, so the right
			// end of the rule stays slightly visible (~70% of a full fade).
			fadeEnd := lipgloss.Blend2D(10, 1, 0, frameC, blockBg)[7]
			grad = lipgloss.Blend2D(cardWidth, 1, 0, frameC, fadeEnd)
		}
		ruleColor := func(col int) color.Color {
			if fade {
				return grad[col]
			}
			return frameC
		}

		title := " " + r.heading + " "
		titleStart, lw := 1, ansi.StringWidth(title)
		titleRunes := []rune(title)

		// Build the top rule column-by-column so the title stays accent while the
		// surrounding dashes follow ruleColor.
		var top strings.Builder
		for col := 0; col < cardWidth; col++ {
			if col >= titleStart && col < titleStart+lw {
				top.WriteString(inline(on(blockBg, accent).Bold(true)).Render(string(titleRunes[col-titleStart])))
				continue
			}
			top.WriteString(inline(on(blockBg, ruleColor(col))).Render("─"))
		}
		var bot strings.Builder
		for col := 0; col < cardWidth; col++ {
			bot.WriteString(inline(on(blockBg, ruleColor(col))).Render("─"))
		}

		out := []string{fillLine(top.String(), cardWidth, blockBg)}
		for _, s := range r.steps {
			out = append(out, fillLine("  "+stepLine(s, blockBg), cardWidth, blockBg))
		}
		return append(out, fillLine(bot.String(), cardWidth, blockBg))

	default: // CodeblockWell
		// The baseline: flat recessed fill, title on the same surface.
		row := func(content string) string {
			return lipgloss.NewStyle().Background(panelBg).Width(cardWidth).Render("  " + content)
		}
		title := row(inline(on(panelBg, accent).Bold(true)).Render(r.heading))
		out := []string{row(""), title, row("")}
		for _, s := range r.steps {
			out = append(out, row(stepLine(s, panelBg)))
		}
		return append(out, row(""))
	}
}

// envRow: tool versions plus git status (branch in amber).
func envRow() string {
	item := func(label, value string) string {
		return inline(on(bg, dim)).Render(label+" ") +
			inline(on(bg, fg).Bold(true)).Render(value+"   ")
	}
	var row string
	for _, t := range envTools {
		row += item(t.label, t.value)
	}
	row += inline(on(bg, dim)).Render("git ") +
		inline(on(bg, amber).Bold(true)).Render(gitBranch) +
		inline(on(bg, muted)).Render(gitStatus)
	return fill(bg).Width(cardWidth).Render(row)
}

// divider: a quieter echo of the banner underline — dashed, dimmed accent peak.
func divider() string {
	return glowRule("┄", dividerPeak)
}

// glowRule renders a full-width rule with a soft symmetric glow: it blends
// bg → peak → bg, so the color peaks in the center and dissolves at both ends.
// `char` and `peak` control how pronounced it is.
func glowRule(char string, peak color.Color) string {
	grad := lipgloss.Blend2D(cardWidth, 1, 0, bg, peak, bg)
	var b strings.Builder
	for col := 0; col < cardWidth; col++ {
		b.WriteString(lipgloss.NewStyle().Foreground(grad[col]).Background(bg).Render(char))
	}
	return b.String()
}

// shortcutLine keeps the three discoverability commands visible without bringing
// back the footer bar. It is quiet, right-aligned, and uses compact aliases.
func shortcutLine() string {
	item := func(command, alias string) string {
		return inline(on(bg, muted)).Bold(true).Render(command) +
			inline(on(bg, dim)).Render(" ("+alias+")")
	}
	content := item("help", "?") +
		inline(on(bg, dim)).Render("  ·  ") +
		item("menu", "m") +
		inline(on(bg, dim)).Render("  ·  ") +
		item("docs", "d")
	return fill(bg).Width(cardWidth).Align(lipgloss.Right).Render(content)
}

// sectionOpen renders the opening lines of a section (the divider + heading),
// dispatching on the configured HeadingStyle. It returns the lines so callers
// can append their own body beneath.
func sectionOpen(text string, hs HeadingStyle) []string {
	switch hs {
	case HeadingInlineC:
		// The heading breaks the divider rule in the center.
		return []string{headingRule(text, -1)}
	case HeadingInlineL:
		// The heading breaks the divider rule near the left.
		return []string{headingRule(text, 1)}
	default: // HeadingSpine
		// A plain divider, then the heading on its own line with an accent spine.
		spine := fill(bg).Width(cardWidth).Render(
			inline(on(bg, accent)).Render("▌ ") +
				inline(on(bg, fg).Bold(true)).Render(text),
		)
		return []string{divider(), blank(), spine}
	}
}

// headingRule is a divider whose glow "breaks" to make room for an emphasized
// heading. `start` is the column the text begins at; pass -1 to center it.
// Centered labels get a symmetric glow (bg �� peak → bg); left-positioned labels
// get a one-sided fade (peak → bg) so the line is brightest beside the text.
func headingRule(text string, start int) string {
	label := " " + text + " " // padding so the glow never touches the text
	runes := []rune(label)
	lw := ansi.StringWidth(label)
	var grad []color.Color
	if start < 0 {
		start = max((cardWidth-lw)/2, 0)
		grad = lipgloss.Blend2D(cardWidth, 1, 0, bg, dividerPeak, bg)
	} else {
		grad = lipgloss.Blend2D(cardWidth, 1, 0, dividerPeak, bg)
	}
	var b strings.Builder
	for col := 0; col < cardWidth; col++ {
		if col >= start && col < start+lw {
			if r := runes[col-start]; r == ' ' {
				b.WriteString(onBg(" "))
			} else {
				b.WriteString(inline(on(bg, fg).Bold(true)).Render(string(r)))
			}
			continue
		}
		b.WriteString(lipgloss.NewStyle().Foreground(grad[col]).Background(bg).Render("┄"))
	}
	return b.String()
}

// ── tiny lipgloss helpers ──────────────────────────────────────────────────────

// on returns a style with the given background and foreground. Note the order:
// on(background, foreground).
func on(bgC, fgC color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fgC).Background(bgC)
}

func fill(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Background(c) }

func inline(s lipgloss.Style) lipgloss.Style { return s.Inline(true) }

func onBg(s string) string { return fill(bg).Render(s) }

func blank() string { return fill(bg).Width(cardWidth).Render("") }

func join(parts ...string) string { return strings.Join(parts, "\n") }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fillLine right-pads a (possibly styled) line to `width` cells with color `c`.
func fillLine(line string, width int, c color.Color) string {
	if w := lipgloss.Width(line); w < width {
		return line + fill(c).Render(strings.Repeat(" ", width-w))
	}
	return line
}

// place horizontally centers the card within the terminal width.
func place(body string, termW int) string {
	offset := max((termW-cardWidth)/2, 0)
	ws := fill(bg)
	var b strings.Builder
	b.WriteString(ws.Width(termW).Render(""))
	b.WriteByte('\n')
	for _, line := range strings.Split(body, "\n") {
		left := ""
		if offset > 0 {
			left = ws.Width(offset).Render("")
		}
		b.WriteString(lipgloss.PlaceHorizontal(termW, lipgloss.Left, left+line, lipgloss.WithWhitespaceStyle(ws)))
		b.WriteByte('\n')
	}
	b.WriteString(ws.Width(termW).Render(""))
	b.WriteByte('\n')
	return b.String()
}
