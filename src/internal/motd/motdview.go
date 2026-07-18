package motd

import (
	"strings"

	"prelude/pkg/ui"
)

// MOTDView is the top-level presentational composer, following React-style
// one-component-per-file composition: it assembles named child components
// without owning rendering state.
type MOTDView struct{ r renderer }

// Render paints the MOTD in its terminal window.
func (v MOTDView) Render() string {
	output := ui.Window{
		Context:      v.r.windowUI,
		Width:        v.r.terminalWidth,
		Offset:       v.r.horizontalOffset,
		TopMargin:    v.r.cfg.Margin.Top,
		BottomMargin: v.r.cfg.Margin.Bottom,
	}.Render(v.renderBody())
	if !v.r.cfg.ClearScreen {
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

	// Fill-above: emit full-width painted rows for every terminal line the
	// MOTD body does not occupy. Placed before the body, they push the MOTD
	// toward the bottom of the terminal so the shell prompt lands directly
	// under it. Each row carries the window background SGR explicitly, so
	// non-BCE terminals still see the fill. The last fill row ends with \n
	// so the body starts on a fresh row.
	fillAbove := ""
	if !v.r.st.windowTransparent {
		fillRows := v.r.terminalHeight - bodyRows - 1
		if fillRows > 0 {
			row := v.r.st.windowFill.Width(v.r.terminalWidth).Render("")
			fillAbove = strings.Repeat(row+"\n", fillRows)
		}
	}

	return clearScreen + fillAbove + output
}

// renderBody composes three sibling surfaces at one shared card width:
// header → divider on window/default → content container.
func (v MOTDView) renderBody() string {
	var sections []string
	card := ui.Surface{Context: v.r.blockUI, Width: v.r.cardWidth}
	header := HeaderView{r: v.r}

	for range max(v.r.cfg.Padding.Top, 0) {
		sections = append(sections, header.BlankLine())
	}
	if content := header.Render(); content != "" {
		sections = append(sections, content)
	}

	// Give a generated title's divider one painted row of breathing room on
	// each side. Other header variants retain their existing spacing.
	if v.r.cfg.Title != "" {
		sections = append(sections, header.BlankLine(), header.Divider(), card.Blank())
	} else {
		sections = append(sections, header.Divider())
	}

	h := v.r.cfg.Header
	shortcuts := (Shortcuts{r: v.r}).Render()
	if h.Tagline != "" || h.Subtitle != "" || shortcuts != "" {
		sections = append(sections, strings.Join((Activation{r: v.r}).Render(h.Tagline, h.Subtitle, shortcuts), "\n"))
	}

	// Newline after the tagline/subtitle when a generated title is active.
	if v.r.cfg.Title != "" && (h.Tagline != "" || h.Subtitle != "") {
		sections = append(sections, card.Blank())
	}

	if middle := v.renderMiddle(); middle != "" {
		sections = append(sections, middle)
	}

	if footer := (FooterView{r: v.r}).Render(); footer != "" {
		sections = append(sections, card.Blank(), footer)
	}

	// Bottom padding is under the whole card.
	for range max(v.r.cfg.Padding.Bottom, 0) {
		sections = append(sections, card.Blank())
	}

	return card.JoinVertical(sections...)
}

// renderMiddle builds description + env + getting-started, then applies
// side padding. Vertical padding is applied around the whole card in renderBody.
// Links are rendered separately by FooterView so they land at the very bottom.
func (v MOTDView) renderMiddle() string {
	var content ui.Block

	if desc := (Description{r: v.r}).Render(); len(desc) > 0 {
		content.WriteSection(desc)
	}

	if env := (Env{r: v.r}).Render(); len(env) > 0 {
		content.WriteSection(env)
	}

	if started := (GettingStartedView{r: v.r}).Render(); len(started) > 0 {
		content.WriteLines(started)
	}

	return ui.PadBlock(
		strings.TrimSuffix(content.String(), "\n"),
		v.r.cardWidth,
		v.r.cfg.Padding.Left,
		v.r.cfg.Padding.Right,
		v.r.st.blockFill,
	)
}
