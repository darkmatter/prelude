package main

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Layout constants encode hard-coded column geometry — not configuration.
const (
	minimumCardWidth = 10
	headerRightPad   = 2 // keep status off the header edge
)

// block accumulates rendered lines into a single string.
type block struct {
	b strings.Builder
}

func (bl *block) write(line string) {
	bl.b.WriteString(line)
	bl.b.WriteByte('\n')
}

func (bl *block) writeLines(lines []string) {
	for _, l := range lines {
		bl.write(l)
	}
}

// writeSection writes lines followed by one blank row. Empty sections skip entirely.
func (bl *block) writeSection(lines []string) {
	if len(lines) == 0 {
		return
	}
	bl.writeLines(lines)
	bl.write("")
}

func (bl *block) String() string {
	return bl.b.String()
}

// renderer holds precomputed layout for one render pass.
type renderer struct {
	cfg              Config
	st               styles
	terminalWidth    int
	cardWidth        int
	contentWidth     int
	horizontalOffset int
	runtime          Runtime
}

func newRenderer(cfg Config, terminalWidth int, runtime Runtime) renderer {
	cardWidth := resolveCardWidth(cfg.Width, cfg.MaxWidth, terminalWidth)
	padLeft := max(cfg.Padding.Left, 0)
	padRight := max(cfg.Padding.Right, 0)
	r := renderer{
		cfg:           cfg,
		st:            newStyles(cfg),
		terminalWidth: max(terminalWidth, 1),
		cardWidth:     cardWidth,
		contentWidth:  max(cardWidth-padLeft-padRight, 1),
		runtime:       runtime,
	}
	r.horizontalOffset = r.resolveHorizontalOffset()
	return r
}

// resolveCardWidth applies width / maxWidth / terminal policy.
func resolveCardWidth(width, maxWidth, terminalWidth int) int {
	cardWidth := width
	if cardWidth == 0 {
		cardWidth = terminalWidth
	}
	if maxWidth > 0 && cardWidth > maxWidth {
		cardWidth = maxWidth
	}
	return max(cardWidth, minimumCardWidth)
}

func (r renderer) resolveHorizontalOffset() int {
	switch r.cfg.Align {
	case "right":
		return max(r.terminalWidth-r.cardWidth-r.cfg.Margin.Right, 0)
	case "center":
		return max((r.terminalWidth-r.cardWidth)/2+r.cfg.Margin.Left-r.cfg.Margin.Right, 0)
	default:
		return max(r.cfg.Margin.Left, 0)
	}
}

// blankLine is a cardWidth-wide row filled with the block background.
func (r renderer) blankLine() string {
	return r.st.blockFill.Width(r.cardWidth).Render("")
}

// fillCardLine pads a short row to cardWidth with the block background.
func (r renderer) fillCardLine(line string) string {
	w := lipgloss.Width(line)
	if w >= r.cardWidth {
		return line
	}
	pad := r.cardWidth - w
	if r.cfg.Background == "" {
		return line + strings.Repeat(" ", pad)
	}
	return line + r.st.blockFill.Render(strings.Repeat(" ", pad))
}

// fillLine pads content to width with an explicit background color.
func (r renderer) fillLine(content string, width int, bg color.Color) string {
	w := lipgloss.Width(content)
	if w >= width {
		return content
	}
	return content + r.st.fill(bg).Render(strings.Repeat(" ", width-w))
}

// joinCardVertical stacks sections and pads every line to cardWidth.
func (r renderer) joinCardVertical(parts ...string) string {
	var bl block
	for _, part := range parts {
		if part == "" {
			continue
		}
		for _, line := range splitLines(part) {
			bl.write(r.fillCardLine(line))
		}
	}
	return strings.TrimSuffix(bl.String(), "\n")
}

// place indents a card line into the terminal window.
func (r renderer) place(line string) string {
	if r.cfg.WindowBackground == "" {
		return strings.Repeat(" ", r.horizontalOffset) + line
	}
	ws := r.st.windowFill
	offset := ""
	if r.horizontalOffset > 0 {
		offset = ws.Width(r.horizontalOffset).Render("")
	}
	return lipgloss.PlaceHorizontal(r.terminalWidth, lipgloss.Left,
		offset+line,
		lipgloss.WithWhitespaceStyle(ws))
}

// renderWindow places the card body into the terminal with margin rows.
func (r renderer) renderWindow(body string) string {
	var bl block
	for range max(r.cfg.Margin.Top, 0) {
		bl.write(r.st.windowFill.Width(r.terminalWidth).Render(""))
	}
	for _, line := range splitLines(body) {
		bl.write(r.place(line))
	}
	for range max(r.cfg.Margin.Bottom, 0) {
		bl.write(r.st.windowFill.Width(r.terminalWidth).Render(""))
	}
	return bl.String()
}

// padContent applies card-level padding around middle content.
func (r renderer) padContent(contentStr string) string {
	if contentStr == "" {
		var parts []string
		pad := r.cfg.Padding
		for range max(pad.Top, 0) {
			parts = append(parts, r.blankLine())
		}
		for range max(pad.Bottom, 0) {
			parts = append(parts, r.blankLine())
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, "\n")
	}
	pad := r.cfg.Padding
	style := r.st.blockFill.
		Width(r.cardWidth).
		Padding(max(pad.Top, 0), max(pad.Right, 0), max(pad.Bottom, 0), max(pad.Left, 0))
	return style.Render(contentStr)
}

func wrapText(value string, width int) []string {
	if value == "" {
		return []string{""}
	}
	return strings.Split(ansi.Wrap(value, max(width, 1), ""), "\n")
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func join(parts ...string) string {
	return strings.Join(parts, "\n")
}
