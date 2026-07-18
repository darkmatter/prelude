// Command-menu playground — portable snapshot of the menu TUI views.
//
// Self-contained: no imports from menu-tui. Run from this directory:
//
//	go run .
//
// Renders the four surfaces the command menu currently has:
//
//  1. list          — fuzzy picker, first task selected
//  2. list+details  — same list with the selected row expanded
//  3. args          — argument-entry mode for a task with chips
//  4. help          — man-style manual (CONTENTS sidebar + body)
//
// Edit the CONFIG block to reskin; rendering code below should stay stable.
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

const (
	innerWidth = 72
	listRows   = 12
	termH      = 28
	padX       = 2

	// Enable only when the active terminal font includes Nerd Font glyphs.
	nerdFonts = false
)

// Palette — prelude theme hex (portable; mirrors src/prelude/themes.nix).
var (
	bg          = lipgloss.Color("#0e0d11")   // deepest: expanded details + manual body
	body        = lipgloss.Color("#121116")   // picker/form body between bg and chrome
	openSurface = lipgloss.Darken(body, 0.01) // shared input/preview + adjacent half rows
	surface     = lipgloss.Color("#19171d")   // lifted title + keymap chrome
	secondary   = lipgloss.Color("#211f28")   // selection and chips (lifted, not text)
	fg          = lipgloss.Color("#d6d2df")   // violet-tinted white for harmony
	muted       = lipgloss.Color("#8787af")   // periwinkle secondary text
	dim         = lipgloss.Color("#4a4556")   // violet-tinted dim (was neutral #444444)
	border      = lipgloss.Color("#373340")   // violet-tinted rails
	accent      = lipgloss.Color("#ff97d7")   // muted rose: selection + primary accent
	accent2     = lipgloss.Color("#a8cf94")   // sage green counterpoint
	// accent      = lipgloss.Color("#ff97d7")   // hot pink: selection + primary accent
	// accent2     = lipgloss.Color("#afff97")
	errC  = lipgloss.Color("#d94f74") // softened raspberry error
	selFg = lipgloss.Color("#0e0d11") // bg-on-accent for selection contrast
)

const project = "prelude"

type glyphs struct {
	selectRow, prompt, expand, collapse string
	args, ready, command                string
	navigate, details, run              string
}

func glyphSet(useNerdFonts bool) glyphs {
	if useNerdFonts {
		return glyphs{
			selectRow: "", prompt: "", expand: "", collapse: "",
			args: "", ready: "", command: "",
			navigate: " ", details: "", run: "",
		}
	}
	return glyphs{
		selectRow: "❯", prompt: "❯", expand: "⇥", collapse: "⇥",
		args: "◆", ready: "●", command: "$",
		navigate: "↑ ↓", details: "⇥", run: "↵",
	}
}

var icons = glyphSet(nerdFonts)

type task struct {
	group, name, key, desc, details, usage, run string
	examples                                    []string
	args                                        []arg
}

type arg struct {
	token, desc string
	required    bool
	boolean     bool
	options     []string
}

var demoTasks = []task{
	{
		group: "general", name: "motd", key: "m", desc: "reprint the welcome banner",
		run: "motd",
	},
	{
		group: "general", name: "menu", key: "n", desc: "open this command menu",
		run: "menu",
	},
	{
		group: "general", name: "docs", key: "d", desc: "open the project manual",
		run: "docs", details: "Man-style manual with a CONTENTS sidebar. Digits jump to sections; j/k scroll; q quits.",
		usage: "docs", examples: []string{"docs", "d"},
	},
	{
		group: "develop", name: "check", key: "c", desc: "build + render smoke tests",
		run: "nix flake check",
	},
	{
		group: "develop", name: "build", key: "b", desc: "build a flake output",
		run: "nix build", usage: "menu build .#motd",
		details: "Builds a flake output. Pass a target or pick from the chips.",
		args: []arg{
			{
				token: "<target>", desc: "flake output to build", required: true,
				options: []string{".#motd", ".#menu", ".#docs", ".#example-themes"},
			},
		},
	},
	{
		group: "demos", name: "examples", key: "e", desc: "tour every feature demo",
		run: "nix run .#examples",
	},
}

// ══════════════════════════════ STYLES ══════════════════════════════════════

type sty struct {
	sp, frame, sFg, sMuted, sDim, sAccent, sAccent2, sErr       lipgloss.Style
	openSp, openFg, openMuted, openDim, openAccent, openAccent2 lipgloss.Style
	barSp, barMuted                                             lipgloss.Style
	keyChip, kbdChip, optChip                                   lipgloss.Style
	selText, selDim, selChip, selSp                             lipgloss.Style
	windowBg                                                    lipgloss.Style
}

func newSty() sty {
	on := func(bgC, fgC color.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(fgC).Background(bgC)
	}
	return sty{
		sp:       lipgloss.NewStyle().Background(body),
		frame:    on(body, border),
		sFg:      on(body, fg),
		sMuted:   on(body, muted),
		sDim:     on(body, dim),
		sAccent:  on(body, accent),
		sAccent2: on(body, accent2),
		sErr:     on(body, errC),
		// Shared by the open input and executable-preview rows. Tune this one
		// Darken value to adjust their separation from the main body.
		openSp:      lipgloss.NewStyle().Background(openSurface),
		openFg:      on(openSurface, fg),
		openMuted:   on(openSurface, muted),
		openDim:     on(openSurface, dim),
		openAccent:  on(openSurface, accent),
		openAccent2: on(openSurface, accent2),
		barSp:       lipgloss.NewStyle().Background(secondary),
		barMuted:    on(secondary, muted),
		// Prototype keycaps: amber glyphs on the dark surface with left/right
		// border rails only, keeping each badge to one terminal row.
		keyChip: on(body, accent2).Bold(true).
			Border(lipgloss.RoundedBorder(), false, true, false, true).
			BorderForeground(border),
		kbdChip: on(secondary, fg).Bold(true),
		optChip: on(secondary, fg),
		// Match the prototype's high-contrast phosphor selection: a full bright
		// row with dark foreground and a dark outlined hotkey chip.
		selText: on(accent, bg),
		selDim:  on(accent, lipgloss.Lighten(bg, 0.18)),
		// Active keycaps avoid Lip Gloss borders: border cells can fall back to
		// the terminal background and cut black bars through the green row.
		selChip:  on(accent, bg).Bold(true),
		selSp:    lipgloss.NewStyle().Background(accent),
		windowBg: lipgloss.NewStyle().Background(bg),
	}
}

func (s sty) bar(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Background(secondary)
}

func (s sty) inset(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c).Background(bg)
}

// ══════════════════════════════ FRAME ═══════════════════════════════════════

func paint(st sty, content string, filler lipgloss.Style, inner int) string {
	content = filler.MaxWidth(inner).Width(inner).Render(content)
	v := st.frame.Render("│")
	return v + content + v
}

func frameTop(st sty, inner int) string {
	return st.frame.Render("╭" + strings.Repeat("─", inner) + "╮")
}
func frameDiv(st sty, inner int) string {
	return st.frame.Render("├" + strings.Repeat("─", inner) + "┤")
}
func frameBottom(st sty, inner int) string {
	return st.frame.Render("╰" + strings.Repeat("─", inner) + "��")
}

func blank(st sty, inner int) string { return paint(st, "", st.sp, inner) }

func letterSpace(s string) string {
	runes := []rune(strings.ToUpper(s))
	out := make([]string, len(runes))
	for i, r := range runes {
		out[i] = string(r)
	}
	return strings.Join(out, "")
}

// mutedTitleRow previews the title as it would appear inside the application's
// existing chrome: menu-background surface, quiet foreground, no nested bar.
func mutedTitleRow(st sty, title string, inner int) string {
	width := inner + 2
	// Lower-half blocks provide a half-cell transition from the page background
	// into the title surface, mirroring the footer's upper-half-block padding.
	halfPad := lipgloss.NewStyle().
		Foreground(surface).
		Background(bg).
		Render(strings.Repeat("▄", width))
	row := lipgloss.NewStyle().
		Foreground(muted).
		Background(surface).
		Width(width).
		MaxWidth(width).
		Align(lipgloss.Center).
		Render(ansi.Truncate(title, width-2*padX-2, "…"))
	// Upper-half blocks extend the header by half a cell before the open input
	// row begins on the darker body surface.
	bottomHalfPad := lipgloss.NewStyle().
		Foreground(surface).
		Background(openSurface).
		Render(strings.Repeat("▀", width))
	return lipgloss.JoinVertical(lipgloss.Left, halfPad, row, bottomHalfPad)
}

func statusBar(st sty, hints [][2]string, status string, inner int) string {
	// Match the original phosphor prototype: a dark continuous keymap surface,
	// outlined-looking key labels, muted descriptions, and a bright live state.
	width := inner + 2
	sp := lipgloss.NewStyle().Background(surface)
	key := lipgloss.NewStyle().Foreground(accent2).Background(bg).Bold(true)
	mutedText := lipgloss.NewStyle().Foreground(muted).Background(surface)
	var b strings.Builder
	b.WriteString(sp.Render(strings.Repeat(" ", padX)))
	for i, h := range hints {
		if i > 0 {
			b.WriteString(mutedText.Render("  "))
		}
		b.WriteString(key.Render(" "+h[0]+" ") + sp.Render(" ") + mutedText.Render(h[1]))
	}
	left := b.String()
	statusColor := accent
	if strings.Contains(status, "args") {
		statusColor = accent2
	}
	right := lipgloss.NewStyle().Foreground(statusColor).Background(surface).Bold(true).Render(status) +
		sp.Render(strings.Repeat(" ", padX))
	available := max(width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	line := left + sp.Render(strings.Repeat(" ", available)) + right
	keymap := sp.Width(width).MaxWidth(width).Render(ansi.Truncate(line, width, ""))
	// Preserve frameBottom above us, then transition from the body into the
	// footer over half a cell. The matching lower transition returns to page bg.
	topHalfPad := lipgloss.NewStyle().
		Foreground(surface).
		Background(openSurface).
		Render(strings.Repeat("▄", width))
	bottomHalfPad := lipgloss.NewStyle().
		Foreground(surface).
		Background(bg).
		Render(strings.Repeat("▀", width))
	return lipgloss.JoinVertical(lipgloss.Left, topHalfPad, keymap, bottomHalfPad)
}

func promptLine(st sty, taskName, input string, inner int) string {
	// Match the prototype's shell-like filter line: contextual path/task,
	// amber caret, and quiet input on the normal content surface.
	line := st.openSp.Render(strings.Repeat(" ", padX))
	context := "~/" + project
	if taskName != "" {
		context = taskName
	}
	line += st.openMuted.Render(context) +
		st.openSp.Render(" ") +
		st.openAccent2.Bold(true).Render(icons.prompt) +
		st.openSp.Render(" ")

	if input == "" {
		line += st.openDim.Render("type to filter commands…")
	} else {
		line += st.openFg.Render(input)
	}
	// Input sits outside the frame, matching the open command-preview region.
	return st.openSp.Width(inner + 2).MaxWidth(inner + 2).Render(line)
}

// ══════════════════════════════ LIST ════════════════════════════════════════

func renderList(st sty, expanded bool) string {
	inner := innerWidth
	title := fmt.Sprintf("%s — command menu", project)
	rows := listRowsContent(st, expanded, inner)

	parts := []string{
		mutedTitleRow(st, title, inner),
		promptLine(st, "", "", inner),
		frameTop(st, inner),
	}
	parts = append(parts, rows...)
	parts = append(parts,
		statusBar(st, [][2]string{
			{icons.navigate, "navigate"}, {icons.details, "details"}, {icons.run, "run"}, {"esc", "clear"},
		}, icons.ready+" ready", inner),
	)
	return strings.Join(parts, "\n")
}

func listRowsContent(st sty, expanded bool, inner int) []string {
	nameW := 4
	for _, t := range demoTasks {
		nameW = max(nameW, lipgloss.Width(t.name))
	}
	nameW += 2

	// Select "docs" (index 2) so expanded details are interesting.
	sel := 2

	var lines []string
	lastGroup := "\x00"
	for i, t := range demoTasks {
		if t.group != lastGroup {
			lastGroup = t.group
			if t.group != "" {
				// paint adds the frame's left rail; subtract that cell so group labels
				// align with the unframed ~/prelude context in the input row.
				label := st.sp.Render(strings.Repeat(" ", max(padX-1, 0))) + st.sMuted.Render(letterSpace(t.group))
				lines = append(lines, paint(st, label, st.sp, inner))
			}
		}
		active := i == sel
		lines = append(lines, renderRow(st, t, active, nameW, inner, expanded))
		if active && expanded {
			lines = append(lines, renderDetails(st, t, inner)...)
		}
	}
	lines = append(lines, blank(st, inner))

	// Collapsed menus retain a stable minimum height. Expanded menus grow to
	// reveal the complete disclosure block, then shrink back when it collapses.
	targetH := listRows
	if expanded {
		targetH = max(targetH, len(lines))
	} else if len(lines) > targetH {
		lines = lines[:targetH]
	}
	for len(lines) < targetH {
		lines = append(lines, blank(st, inner))
	}
	return lines
}

func renderRow(st sty, t task, active bool, nameW, inner int, expanded bool) string {
	keyLabel := ""
	if t.key != "" {
		// The glyph plus two side borders occupies the existing three-cell slot.
		keyLabel = t.key
	}
	// Reserve the right lane for capabilities that are actually present. Avoid
	// repeating args on ordinary commands; only argument-taking tasks advertise it.
	marker := ""
	if len(t.args) > 0 {
		marker = icons.args + " args"
	} else if t.details != "" {
		marker = icons.expand + " details"
	}
	if active && t.details != "" {
		if expanded {
			marker = icons.collapse + " less"
		} else {
			marker = icons.expand + " more"
		}
	}

	if active {
		// Three compact columns: caret, exact-width shortcut, then command.
		caretCol := lipgloss.NewStyle().Foreground(bg).Background(accent).Bold(true).
			Width(2).Render(icons.selectRow)
		shortcutCol := st.selSp.Width(3).Render("")
		if t.key != "" {
			// Styled glyph rails preserve the outlined keycap while every cell keeps
			// the active row's phosphor background.
			shortcutCol = st.selChip.Render("│" + keyLabel + "│")
		}
		name := st.selText.Bold(true).Width(nameW).Render(t.name)
		used := padX + 2 + 3 + 1 + nameW + 1 + lipgloss.Width(marker) + 1 + padX
		desc := st.selText.Render(ansi.Truncate(t.desc, max(inner-used, 4), "…"))
		line := st.selSp.Render(strings.Repeat(" ", padX)) + caretCol +
			shortcutCol + st.selSp.Render(" ") + name + st.selSp.Render(" ") + desc
		pad := inner - lipgloss.Width(line) - lipgloss.Width(marker) - padX
		markerStyle := st.selDim
		if len(t.args) > 0 {
			markerStyle = lipgloss.NewStyle().Foreground(bg).Background(accent).Bold(true)
		}
		line += st.selSp.Render(strings.Repeat(" ", max(pad, 1))) + markerStyle.Render(marker) +
			st.selSp.Render(strings.Repeat(" ", padX))
		return paint(st, line, st.selSp, inner)
	}

	caretCol := st.sp.Width(2).Render("")
	shortcutCol := st.sp.Width(3).Render("")
	if t.key != "" {
		shortcutCol = st.keyChip.Render(keyLabel)
	}
	used := padX + 2 + 3 + 1 + nameW + 1 + lipgloss.Width(marker) + 1 + padX
	desc := st.sMuted.Render(ansi.Truncate(t.desc, max(inner-used, 4), "…"))
	line := st.sp.Render(strings.Repeat(" ", padX)) + caretCol + shortcutCol + st.sp.Render(" ") +
		st.sFg.Bold(true).Width(nameW).Render(t.name) + st.sp.Render(" ") + desc
	pad := inner - lipgloss.Width(line) - lipgloss.Width(marker) - padX
	markerStyle := st.sDim
	if len(t.args) > 0 {
		markerStyle = st.sAccent2
	}
	line += st.sp.Render(strings.Repeat(" ", max(pad, 1))) + markerStyle.Render(marker) +
		st.sp.Render(strings.Repeat(" ", padX))
	return paint(st, line, st.sp, inner)
}

func renderDetails(st sty, t task, inner int) []string {
	insetSp := lipgloss.NewStyle().Background(bg)
	// Align disclosure content with the keycap after the compact caret lane.
	detailIndent := padX + 2
	indent := insetSp.Render(strings.Repeat(" ", detailIndent))
	paintInset := func(content string) string {
		// Keep expanded details open vertically, but restore quiet side rails so
		// the disclosure remains aligned with the surrounding picker frame.
		panel := insetSp.Width(inner).MaxWidth(inner).Render(content)
		return st.frame.Render("│") + panel + st.frame.Render("│")
	}
	var out []string
	out = append(out, paintInset(""))
	if t.details != "" {
		wrapW := inner - detailIndent - padX
		for _, l := range strings.Split(ansi.Wordwrap(t.details, wrapW, ""), "\n") {
			out = append(out, paintInset(indent+st.inset(muted).Render(l)))
		}
	}
	if t.usage != "" {
		out = append(out, paintInset(""),
			paintInset(indent+st.inset(accent).Render(icons.command+" ")+st.inset(fg).Render(t.usage)))
	}
	for _, ex := range t.examples {
		out = append(out, paintInset(indent+st.inset(dim).Render("example ")+
			st.inset(accent).Render(icons.selectRow+" ")+st.inset(muted).Render(ex)))
	}
	out = append(out, paintInset(""))
	return out
}

// ══════════════════════════════ ARGS ════════════════════════════════════════

func renderArgs(st sty) string {
	inner := innerWidth
	t := demoTasks[4] // build — has args
	title := fmt.Sprintf("%s %s — enter arguments", project, t.name)

	tokenW := 4
	for _, a := range t.args {
		tokenW = max(tokenW, lipgloss.Width(a.token))
	}

	var body []string
	body = append(body, blank(st, inner))
	body = append(body, paint(st, st.sp.Render(strings.Repeat(" ", padX))+st.sMuted.Render(letterSpace("arguments")), st.sp, inner))
	body = append(body, blank(st, inner))

	focusChip := 1 // highlight .#menu
	chipIdx := 0
	for _, a := range t.args {
		tag, tagStyle := "OPTIONAL", st.sDim
		if a.required {
			tag, tagStyle = "REQUIRED", st.sErr
		}
		if a.boolean {
			tag = "FLAG"
		}
		row := st.sp.Render(strings.Repeat(" ", padX)) +
			st.sAccent.Bold(true).Width(tokenW).Render(a.token) + st.sp.Render("  ") +
			tagStyle.Width(8).Render(tag) + st.sp.Render("  ") +
			st.sMuted.Render(a.desc)
		body = append(body, paint(st, row, st.sp, inner))

		if len(a.options) > 0 {
			var chips []string
			for _, opt := range a.options {
				label := " " + opt + " "
				if chipIdx == focusChip {
					// Focused options use the phosphor selection treatment; the other
					// chips remain on the quieter secondary surface.
					chips = append(chips, lipgloss.NewStyle().
						Foreground(bg).Background(accent).Bold(true).Render(label))
				} else {
					chips = append(chips, st.optChip.Render(label))
				}
				chipIdx++
			}
			row := st.sp.Render(strings.Repeat(" ", padX+tokenW+2)) +
				strings.Join(chips, st.sp.Render(" "))
			body = append(body, paint(st, row, st.sp, inner))
		}
		body = append(body, blank(st, inner))
	}

	h := listRows - 3
	for len(body) < h {
		body = append(body, blank(st, inner))
	}
	if len(body) > h {
		body = body[:h]
	}

	preview := st.openSp.Render(strings.Repeat(" ", padX)) +
		st.openAccent.Render(icons.command+" ") +
		st.openFg.Render(t.run+" .#menu")
	// The executable preview is an open full-width region, like expanded
	// details: no left/right frame rails before it transitions into the footer.
	openPreview := st.openSp.Width(inner + 2).MaxWidth(inner + 2).Render(preview)

	parts := []string{
		mutedTitleRow(st, title, inner),
		promptLine(st, t.name, ".#menu", inner),
		frameTop(st, inner),
	}
	parts = append(parts, body...)
	parts = append(parts,
		frameBottom(st, inner),
		openPreview,
		statusBar(st, [][2]string{
			{icons.details, "chips"}, {icons.run, "run"}, {"esc", "back"},
		}, icons.args+" args", inner),
	)
	return strings.Join(parts, "\n")
}

// ══════════════════���═══════════ HELP ══════════════════════════════════���═════

func renderHelp(st sty) string {
	// Full-bleed help: sidebar + body, matching menu-tui/docs viewer.
	sections := []string{"name", "synopsis", "description", "options", "commands", "examples", "see also"}
	active := 0
	width := max(innerWidth+18, 90)
	height := termH

	side := lipgloss.Width("CONTENTS")
	for _, s := range sections {
		side = max(side, lipgloss.Width(s)+2)
	}
	side += 4
	bodyW := max(width-side-1, 40)
	textW := min(bodyW-2, 72)
	viewH := max(height-2, 1)

	doc := helpDoc(st, textW)
	insetSp := lipgloss.NewStyle().Background(bg)
	padSide := func(s string) string {
		return lipgloss.NewStyle().Background(surface).MaxWidth(side).Width(side).Render(s)
	}
	padBody := func(s string) string {
		return insetSp.MaxWidth(bodyW).Width(bodyW).Render(s)
	}
	div := st.inset(border)
	frame := lipgloss.NewStyle().Foreground(border).Background(surface)

	rows := make([]string, 0, height)
	rows = append(rows, frame.Render(strings.Repeat("─", side))+
		div.Render("┬"+strings.Repeat("─", bodyW)))

	for r := 0; r < viewH; r++ {
		junction := "│"
		var sideCell string
		switch r {
		case 0:
			sideCell = padSide("")
		case 1:
			sideCell = padSide(lipgloss.NewStyle().Background(surface).Render("  ") +
				lipgloss.NewStyle().Foreground(muted).Background(surface).Render("CONTENTS"))
		case 2:
			sideCell = frame.Render(strings.Repeat("─", side))
			junction = "┤"
		default:
			sideCell = helpSidebarItem(st, sections, r-3, active, side, padSide)
		}
		body := ""
		if r < len(doc) {
			body = doc[r]
		}
		rows = append(rows, sideCell+div.Render(junction)+padBody(body))
	}

	// Status bar
	fgC := fg
	sp := lipgloss.NewStyle().Background(fgC)
	txt := lipgloss.NewStyle().Background(fgC).Foreground(bg)
	left := sp.Render("  ") + txt.Bold(true).Render("NORMAL") +
		txt.Render(" :"+sections[active]) +
		txt.Faint(true).Render("  ·  1-7 jump · j/k scroll · q quit")
	right := txt.Faint(true).Render("top") + sp.Render("  ")
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	bar := left + sp.Render(strings.Repeat(" ", max(pad, 0))) + right
	rows = append(rows, ansi.Truncate(bar, width, ""))
	return strings.Join(rows, "\n")
}

func helpSidebarItem(st sty, sections []string, i, active, sideW int, padSide func(string) string) string {
	if i < 0 || i >= len(sections) {
		return padSide("")
	}
	title := sections[i]
	if i == active {
		line := st.barSp.Render("  ") + st.bar(accent).Render(icons.selectRow) +
			st.barSp.Render(" ") + st.bar(fg).Render(title)
		return st.barSp.MaxWidth(sideW).Width(sideW).Render(line)
	}
	line := lipgloss.NewStyle().Background(surface).Render("  ") +
		lipgloss.NewStyle().Foreground(dim).Background(surface).Render(fmt.Sprintf("%d", i+1)) +
		lipgloss.NewStyle().Background(surface).Render(" ") +
		lipgloss.NewStyle().Foreground(muted).Background(surface).Render(title)
	return padSide(line)
}

func helpDoc(st sty, textW int) []string {
	sp := func(n int) string {
		return lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", n))
	}
	var out []string
	blank := func() { out = append(out, "") }
	section := func(title string) {
		if len(out) > 0 && out[len(out)-1] != "" {
			blank()
		}
		out = append(out, sp(2)+st.inset(accent).Bold(true).Render(strings.ToUpper(title)))
	}
	para := func(c color.Color, indent int, text string) {
		for _, l := range strings.Split(ansi.Wordwrap(text, max(textW-indent, 16), ""), "\n") {
			out = append(out, sp(indent)+st.inset(c).Render(l))
		}
	}
	entry := func(term, desc string) {
		out = append(out, sp(4)+st.inset(accent2).Bold(true).Render(term))
		para(muted, 6, desc)
		blank()
	}
	cmdline := func(cmd string) {
		out = append(out, sp(4)+st.inset(accent).Render(icons.command+" ")+st.inset(fg).Render(cmd))
	}

	blank()
	section("name")
	out = append(out, sp(4)+
		st.inset(accent2).Bold(true).Render(project)+
		st.inset(fg).Render(" — devshell UI: welcome banner, command menu, and docs"))
	section("synopsis")
	cmdline("motd | ?")
	cmdline("menu | m [task]")
	cmdline("docs | d")
	cmdline("help")
	section("description")
	para(muted, 4, "prelude greets you with a static MOTD, then exposes shortcuts for the session. The interactive menu is a fuzzy-filtered picker over tasks declared in Nix.")
	blank()
	section("options")
	entry("motd, ?", "Reprint the MOTD welcome banner.")
	entry("menu, m", "Open the interactive command picker.")
	entry("docs, d", "Open the project docs viewer.")
	entry("help", "Show this manual.")
	section("commands")
	entry("check (c)", "Build packages and run render smoke tests.")
	entry("build (b)", "Build a flake output; opens argument entry when no target is given.")
	section("examples")
	cmdline("menu")
	para(dim, 6, "open the interactive picker")
	blank()
	cmdline("docs")
	para(dim, 6, "open this manual; press 5 to jump to COMMANDS")
	blank()
	section("see also")
	para(muted, 4, "README.md — module options, themes, and downstream usage.")
	blank()
	return out
}

// ════════════���═════════════════ MAIN ═══════════════════════════════���════════

func hexRGB(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

// wings renders the left and right horizontal fades: one Blend1D color per
// column, ordered outer → inner. The outermost column is exactly the terminal
// color so the window melts into the screen edge; the window-background
// endpoint is trimmed so the innermost column stays one step short of the
// plateau (same recipe as motd's windowWingColors).
func wings(termBg color.Color, width int) (left, right string) {
	colors := lipgloss.Blend1D(width+1, termBg, bg)[:width]
	var lb, rb strings.Builder
	for i := range colors {
		lb.WriteString(lipgloss.NewStyle().Background(colors[i]).Render(" "))
		rb.WriteString(lipgloss.NewStyle().Background(colors[len(colors)-1-i]).Render(" "))
	}
	return lb.String(), rb.String()
}

func main() {
	st := newSty()
	termW := 100
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termW = w
	}

	// Query the real terminal background. When available, fade the window
	// background into it horizontally: gradient wings on both sides of every
	// row, sized from the space left over beside the content.
	// TERM_BG=#rrggbb overrides the query, for eyeballing the fade against a
	// terminal color that isn't nearly identical to the window background.
	termBg, termBgErr := lipgloss.BackgroundColor(os.Stdin, os.Stdout)
	if v := os.Getenv("TERM_BG"); v != "" {
		termBg, termBgErr = lipgloss.Color(v), nil
	}
	// Blend only at the outermost edge: a narrow wing band, never more than a
	// third of the side margin, so a flat window-bg plateau always separates
	// the fade from the content.
	wingW := 0
	if termBgErr == nil {
		wingW = min(6, max((termW-(innerWidth+2))/6, 0))
	}
	innerW := termW - 2*wingW

	label := func(s string) string {
		return lipgloss.NewStyle().Foreground(dim).Background(bg).
			Render("── " + s + " ")
	}
	place := func(body string) string {
		var out []string
		for _, line := range strings.Split(body, "\n") {
			pad := max(innerW-lipgloss.Width(line), 0)
			out = append(out, line+st.windowBg.Render(strings.Repeat(" ", pad)))
		}
		return strings.Join(out, "\n")
	}

	views := []struct {
		name string
		body string
	}{
		{"list · picker, selection on docs", renderList(st, false)},
		{"list · details expanded (" + icons.expand + ")", renderList(st, true)},
		{"args · build task with chips", renderArgs(st)},
		{"help · man-style manual", renderHelp(st)},
	}

	var parts []string
	if termBgErr == nil {
		parts = append(parts, place(label(fmt.Sprintf("terminal bg %s · fading window bg %s into it", hexRGB(termBg), hexRGB(bg)))))
	} else {
		parts = append(parts, place(label("terminal bg unavailable ("+termBgErr.Error()+") · hard edges")))
	}
	for _, v := range views {
		parts = append(parts,
			st.windowBg.Width(innerW).Render(""), st.windowBg.Width(innerW).Render(""),
			place(label(v.name)),
			st.windowBg.Width(innerW).Render(""),
			place(v.body),
		)
	}
	out := strings.Join(parts, "\n")
	if wingW > 0 {
		left, right := wings(termBg, wingW)
		lines := strings.Split(out, "\n")
		for i := range lines {
			lines[i] = left + lines[i] + right
		}
		out = strings.Join(lines, "\n")
	}
	fmt.Print(out + "\n")
}
