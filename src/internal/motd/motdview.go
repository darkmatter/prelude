package motd

import (
	"strings"

	"prelude/pkg/ui"
)

// MOTDView is the top-level presentational composer. It assembles header,
// middle, and footer sections through shallow adapters in sections.go, keeping
// layout policy in one place.
type MOTDView struct{ r renderer }

// Render paints the MOTD in its terminal window.
func (v MOTDView) Render() string {
	output := ui.Window{
		Context:      v.r.windowUI,
		Width:        v.r.terminalWidth,
		Offset:       v.r.horizontalOffset,
		TopMargin:    v.r.model.Margin.Top,
		BottomMargin: v.r.model.Margin.Bottom,
	}.Render(v.renderBody())
	if !v.r.model.Config.ClearScreen {
		return output
	}
	// Count emitted rows so the scroll-fill can paint exactly the remaining
	// terminal height without over-scrolling.
	bodyRows := strings.Count(output, "\n")

	// Painted erase: sets the window background SGR, then Erase Display.
	// Terminals with Background Color Erase (BCE) fill every cell; terminals
	// without it (Warp) ignore the SGR during erase, so we also scroll-fill.
	clearScreen := "\x1b[2J\x1b[H"
	if !v.r.st.windowTransparent {
		clearScreen = v.r.st.windowFill.Render(clearScreen)
	}

	// Fill-above: emit full-width rows for every terminal line the MOTD body
	// does not occupy. Placed before the body, they push the MOTD toward the
	// bottom of the terminal so the shell prompt lands directly under it.
	// This is layout, not coloring — the rows are emitted regardless of
	// window background. When a background is set each row carries the SGR
	// explicitly so non-BCE terminals still see the fill; when transparent
	// the rows are blank but still occupy vertical space. The last fill row
	// ends with \n so the body starts on a fresh row.
	fillAbove := ""
	fillRows := v.r.terminalHeight - bodyRows - 1
	if fillRows > 0 {
		row := v.r.st.windowFill.Width(v.r.terminalWidth).Render("")
		fillAbove = strings.Repeat(row+"\n", fillRows)
	}

	return clearScreen + fillAbove + output
}

// renderBody collapses the MOTD into three sibling sections at one shared
// card width: Header → Body → Footer. Empty sections are omitted entirely so
// spacing never shells around absent content; a single blank separates live
// sections. Outer card padding still wraps the whole stack.
func (v MOTDView) renderBody() string {
	card := ui.Surface{Context: v.r.blockUI, Width: v.r.cardWidth}

	var sections []string
	for range max(v.r.model.Padding.Top, 0) {
		sections = append(sections, card.Blank())
	}

	// Collapse empty sections: only paint Header/Body/Footer when they have
	// content, and insert one blank between consecutive live sections.
	live := []string{
		v.renderHeaderSection(),
		v.renderBodySection(),
		v.renderFooterSection(),
	}
	first := true
	for _, section := range live {
		if section == "" {
			continue
		}
		if !first {
			sections = append(sections, card.Blank())
		}
		sections = append(sections, section)
		first = false
	}

	for range max(v.r.model.Padding.Bottom, 0) {
		sections = append(sections, card.Blank())
	}

	return card.JoinVertical(sections...)
}

// renderHeaderSection owns the wordmark/title chrome, divider, and activation
// strip (tagline/subtitle/shortcuts). Returns "" when nothing would paint.
func (v MOTDView) renderHeaderSection() string {
	header := HeaderView{r: v.r}
	sec := sections{r: v.r}
	card := ui.Surface{Context: v.r.blockUI, Width: v.r.cardWidth}
	var parts []string

	if content := header.Render(); content != "" {
		parts = append(parts, content)
	}

	// Give a generated title's divider one painted row of breathing room on
	// each side. Other header variants retain their existing spacing.
	if v.r.model.Config.Title != "" {
		parts = append(parts, header.BlankLine(), header.Divider(), card.Blank())
	} else if div := header.Divider(); div != "" {
		parts = append(parts, div)
	}

	h := v.r.model.Config.Header
	shortcuts := sec.shortcuts()
	if h.Tagline != "" || h.Subtitle != "" || shortcuts != "" {
		parts = append(parts, strings.Join(sec.activation(h.Tagline, h.Subtitle, shortcuts), "\n"))
	}

	// Newline after the tagline/subtitle when a generated title is active.
	if v.r.model.Config.Title != "" && (h.Tagline != "" || h.Subtitle != "") {
		parts = append(parts, card.Blank())
	}

	return joinNonEmpty(parts)
}

// renderBodySection owns description + env + getting-started with side
// padding. Returns "" when the middle is empty so Footer can sit directly
// under Header without a hollow shell.
func (v MOTDView) renderBodySection() string {
	return v.renderMiddle()
}

// renderFooterSection owns status badges and terminal links. Returns "" when
// FooterView has nothing to paint.
func (v MOTDView) renderFooterSection() string {
	return (FooterView{r: v.r}).Render()
}

// renderMiddle builds description + env + getting-started, then applies
// side padding. Vertical padding is applied around the whole card in renderBody.
// Links are rendered separately by FooterView so they land at the very bottom.
func (v MOTDView) renderMiddle() string {
	sec := sections{r: v.r}
	var content ui.Block

	if desc := sec.description(); len(desc) > 0 {
		content.WriteSection(desc)
	}

	if env := sec.env(); len(env) > 0 {
		content.WriteSection(env)
	}

	if started := sec.gettingStarted(); len(started) > 0 {
		content.WriteLines(started)
	}

	body := strings.TrimSuffix(content.String(), "\n")
	if body == "" {
		return ""
	}

	return ui.PadBlock(
		body,
		v.r.cardWidth,
		v.r.model.Padding.Left,
		v.r.model.Padding.Right,
		v.r.st.blockFill,
	)
}

func joinNonEmpty(parts []string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "\n")
}
