// Package manual renders and navigates Prelude manuals through one shared viewer.
package manual

import (
	"fmt"
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/shared"
)

// Role selects a palette role for a document span.
type Role uint8

const (
	Foreground Role = iota
	Muted
	Dim
	Accent
	Accent2
)

// Span is one styled fragment in a document block.
type Span struct {
	Role Role
	Text string
	Bold bool
}

// Block is one semantic content row. Wrapped blocks should contain one span.
type Block struct {
	Indent     int
	Wrap       bool
	BlankAfter bool
	Spans      []Span
}

// Section is one sidebar entry and its content.
type Section struct {
	Title  string
	Blocks []Block
}

// Document is the presentation model consumed by the viewer.
type Document struct {
	Sections []Section
}

// SidebarItemsTop is the terminal row occupied by the first sidebar item.
const SidebarItemsTop = 4

type layout struct {
	sideW int
	bodyW int
	textW int
	viewH int
}

type styles struct {
	pal shared.Palette

	surface   color.Color
	secondary color.Color
	bg        color.Color

	surfaceSpace lipgloss.Style
	frame        lipgloss.Style
	surfaceMuted lipgloss.Style
	bodySpace    lipgloss.Style
	activeSpace  lipgloss.Style
}

func newStyles(p shared.Palette) styles {
	h := shared.NewPaletteHelper(p)
	surface := h.Color(string(p.Surface))
	secondary := h.Color(string(p.Secondary))
	bg := h.Color(string(p.Bg))
	return styles{
		pal:          p,
		surface:      surface,
		secondary:    secondary,
		bg:           bg,
		surfaceSpace: lipgloss.NewStyle().Background(surface),
		frame:        h.On(surface, string(p.Border)),
		surfaceMuted: h.On(surface, string(p.Muted)),
		bodySpace:    lipgloss.NewStyle().Background(bg),
		activeSpace:  lipgloss.NewStyle().Background(secondary),
	}
}

func (s styles) onBody(role Role, bold bool) lipgloss.Style {
	fg := s.pal.Fg
	switch role {
	case Muted:
		fg = s.pal.Muted
	case Dim:
		fg = s.pal.Dim
	case Accent:
		fg = s.pal.Accent
	case Accent2:
		fg = s.pal.Accent2
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(fg))).
		Background(s.bg).
		Bold(bold)
}

func (s styles) onActive(fg shared.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(string(fg))).Background(s.secondary)
}

// Viewer owns manual state, navigation, layout, chrome, and presentation.
type Viewer struct {
	document Document
	styles   styles
	width    int
	height   int
	scroll   int
	active   int
}

// New constructs a viewer with the default terminal dimensions.
func New(document Document, palette shared.Palette) Viewer {
	return Viewer{
		document: document,
		styles:   newStyles(palette),
		width:    80,
		height:   24,
	}
}

func (v Viewer) Init() tea.Cmd { return nil }

func (v Viewer) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := v.Handle(msg)
	return next, cmd
}

// Handle applies one Bubble Tea message and returns the updated viewer.
func (v Viewer) Handle(msg tea.Msg) (Viewer, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width, v.height = msg.Width, msg.Height
		v.clamp()
	case tea.KeyPressMsg:
		return v.handleKey(msg)
	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			v.click(msg.X, msg.Y)
		}
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelDown:
			v.scrollBy(3)
		case tea.MouseWheelUp:
			v.scrollBy(-3)
		}
	}
	return v, nil
}

func (v Viewer) handleKey(msg tea.KeyPressMsg) (Viewer, tea.Cmd) {
	l := v.layout()
	switch msg.String() {
	case "q", "esc", "ctrl+c":
		return v, tea.Quit
	case "j", "down", "ctrl+n":
		v.scrollBy(1)
	case "k", "up", "ctrl+p":
		v.scrollBy(-1)
	case "pgdown", "space", "ctrl+d":
		v.scrollBy(l.viewH)
	case "pgup", "b", "ctrl+u":
		v.scrollBy(-l.viewH)
	case "home", "g":
		v.scroll, v.active = 0, 0
	case "end", "G", "shift+g":
		v.scrollBy(1 << 20)
	default:
		if key := msg.String(); len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			v.jumpSection(int(key[0] - '1'))
		}
	}
	return v, nil
}

// View returns the full-screen Bubble Tea view.
func (v Viewer) View() tea.View {
	view := tea.NewView(v.render())
	view.BackgroundColor = v.styles.bg
	view.AltScreen = true
	view.MouseMode = tea.MouseModeCellMotion
	return view
}

func (v Viewer) layout() layout {
	side := lipgloss.Width("CONTENTS")
	for _, section := range v.document.Sections {
		side = max(side, lipgloss.Width(section.Title)+2)
	}
	side += 4
	body := max(v.width-side-1, 20)
	return layout{
		sideW: side,
		bodyW: body,
		textW: min(body-2, 96),
		viewH: max(v.height-2, 1),
	}
}

func (v *Viewer) scrollBy(delta int) {
	l := v.layout()
	lines, starts := v.renderDocument(l.textW)
	maxScroll := max(0, len(lines)-l.viewH)
	v.scroll = min(max(v.scroll+delta, 0), maxScroll)
	v.active = activeAt(starts, v.scroll)
}

func (v *Viewer) jumpSection(index int) {
	if index < 0 || index >= len(v.document.Sections) {
		return
	}
	l := v.layout()
	lines, starts := v.renderDocument(l.textW)
	maxScroll := max(0, len(lines)-l.viewH)
	v.scroll = min(starts[index], maxScroll)
	v.active = index
}

func (v *Viewer) click(x, y int) {
	if x >= v.layout().sideW {
		return
	}
	if index := y - SidebarItemsTop; index >= 0 && index < len(v.document.Sections) {
		v.jumpSection(index)
	}
}

func (v *Viewer) clamp() {
	l := v.layout()
	lines, starts := v.renderDocument(l.textW)
	v.scroll = min(v.scroll, max(0, len(lines)-l.viewH))
	v.active = activeAt(starts, v.scroll)
}

func activeAt(starts []int, offset int) int {
	active := 0
	for index, start := range starts {
		if start <= offset {
			active = index
		}
	}
	return active
}

func (v Viewer) renderDocument(textWidth int) (lines []string, starts []int) {
	textWidth = max(textWidth, 24)
	space := func(count int) string {
		return v.styles.bodySpace.Render(strings.Repeat(" ", count))
	}
	lines = append(lines, "")
	for _, section := range v.document.Sections {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		starts = append(starts, len(lines))
		lines = append(lines, space(2)+v.styles.onBody(Accent, true).Render(strings.ToUpper(section.Title)))
		for _, block := range section.Blocks {
			lines = append(lines, v.renderBlock(block, textWidth, space)...)
			if block.BlankAfter {
				lines = append(lines, "")
			}
		}
	}
	return lines, starts
}

func (v Viewer) renderBlock(block Block, textWidth int, space func(int) string) []string {
	if len(block.Spans) == 0 {
		return nil
	}
	plain := strings.Builder{}
	for _, span := range block.Spans {
		plain.WriteString(span.Text)
	}
	if plain.Len() == 0 {
		return nil
	}
	if block.Wrap {
		role, bold := block.Spans[0].Role, block.Spans[0].Bold
		wrapped := strings.Split(ansi.Wordwrap(plain.String(), max(textWidth-block.Indent, 16), ""), "\n")
		lines := make([]string, len(wrapped))
		for index, line := range wrapped {
			lines[index] = space(block.Indent) + v.styles.onBody(role, bold).Render(line)
		}
		return lines
	}

	var line strings.Builder
	line.WriteString(space(block.Indent))
	for _, span := range block.Spans {
		line.WriteString(v.styles.onBody(span.Role, span.Bold).Render(span.Text))
	}
	return []string{line.String()}
}

func (v Viewer) render() string {
	l := v.layout()
	lines, _ := v.renderDocument(l.textW)
	maxScroll := max(0, len(lines)-l.viewH)
	scroll := min(v.scroll, maxScroll)
	padSide := func(content string) string {
		return v.styles.surfaceSpace.MaxWidth(l.sideW).Width(l.sideW).Render(content)
	}
	padBody := func(content string) string {
		return v.styles.bodySpace.MaxWidth(l.bodyW).Width(l.bodyW).Render(content)
	}
	divider := v.styles.onBody(Foreground, false).Foreground(lipgloss.Color(string(v.styles.pal.Border)))

	rows := make([]string, 0, v.height)
	rows = append(rows, v.styles.frame.Render(strings.Repeat("─", l.sideW))+
		divider.Render("┬"+strings.Repeat("─", l.bodyW)))
	for row := 0; row < l.viewH; row++ {
		junction := "│"
		var side string
		switch row {
		case 0:
			side = padSide("")
		case 1:
			side = padSide(v.styles.surfaceSpace.Render("  ") + v.styles.surfaceMuted.Render("CONTENTS"))
		case 2:
			side = v.styles.frame.Render(strings.Repeat("─", l.sideW))
			junction = "┤"
		default:
			side = v.sidebarItem(row-3, l.sideW, padSide)
		}
		body := ""
		if index := scroll + row; index < len(lines) {
			body = lines[index]
		}
		rows = append(rows, side+divider.Render(junction)+padBody(body))
	}
	rows = append(rows, v.statusBar(scroll, maxScroll))
	return strings.Join(rows, "\n")
}

func (v Viewer) sidebarItem(index, sideWidth int, pad func(string) string) string {
	if index < 0 || index >= len(v.document.Sections) {
		return pad("")
	}
	title := v.document.Sections[index].Title
	if index == v.active {
		line := v.styles.activeSpace.Render("  ") + v.styles.onActive(v.styles.pal.Accent).Render("❯") +
			v.styles.activeSpace.Render(" ") + v.styles.onActive(v.styles.pal.Fg).Render(title)
		return v.styles.activeSpace.MaxWidth(sideWidth).Width(sideWidth).Render(line)
	}
	line := v.styles.surfaceSpace.Render("  ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color(string(v.styles.pal.Dim))).Background(v.styles.surface).Render(fmt.Sprintf("%d", index+1)) +
		v.styles.surfaceSpace.Render(" ") + v.styles.surfaceMuted.Render(title)
	return pad(line)
}

func (v Viewer) statusBar(scroll, maxScroll int) string {
	foreground := lipgloss.Color(string(v.styles.pal.Fg))
	space := lipgloss.NewStyle().Background(foreground)
	text := lipgloss.NewStyle().Background(foreground).Foreground(v.styles.bg)
	position := fmt.Sprintf("%d%%", scroll*100/max(maxScroll, 1))
	switch {
	case maxScroll == 0:
		position = "all"
	case scroll == 0:
		position = "top"
	case scroll >= maxScroll:
		position = "bot"
	}
	section := ""
	if count := len(v.document.Sections); count > 0 {
		section = v.document.Sections[min(v.active, count-1)].Title
	}
	jumpCount := min(len(v.document.Sections), 9)
	left := space.Render("  ") + text.Bold(true).Render("NORMAL") + text.Render(" :"+section)
	if jumpCount > 0 {
		left += text.Faint(true).Render(fmt.Sprintf("  ·  1-%d jump · j/k scroll · q quit", jumpCount))
	}
	right := text.Faint(true).Render(position) + space.Render("  ")
	padding := v.width - lipgloss.Width(left) - lipgloss.Width(right)
	return ansi.Truncate(left+space.Render(strings.Repeat(" ", max(padding, 0)))+right, v.width, "")
}
