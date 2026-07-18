package motd

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

// Activation renders the post-underline activation block: a bold accent title
// plus an optional faint muted subtitle.
type Activation struct {
	r renderer
}

// Render paints the post-underline block. Shortcuts occupy a right-aligned
// lane beside the tagline; when the row cannot fit, the tagline wraps first and
// the shortcut lane remains right-aligned on its own row.
func (x Activation) Render(tagline, subtitle, shortcuts string) []string {
	layout := strings.ToLower(x.r.cfg.Header.TaglineLayout)
	if layout == "" {
		layout = "stack"
	}
	align := lipgloss.Left
	switch strings.ToLower(x.r.cfg.Header.TaglineAlign) {
	case "center":
		align = lipgloss.Center
	case "right":
		align = lipgloss.Right
	}

	title := ""
	if tagline != "" {
		title = ui.Inline(x.r.st.amber).Bold(true).Render(tagline)
	}
	sub := ""
	if subtitle != "" {
		sub = ui.Inline(x.r.st.muted).Faint(true).Render(subtitle)
	}

	place := func(content string) string {
		return ui.PlaceContentLine(content, x.r.cardWidth, x.r.contentWidth, x.r.cfg.Padding.Left, align, x.r.st.blockFill)
	}
	inline := title
	if layout == "inline" && sub != "" {
		if inline != "" {
			inline += ui.Inline(x.r.st.dim).Render("  ·  ")
		}
		inline += sub
		sub = ""
	}

	var out []string
	if shortcuts != "" && inline != "" && lipgloss.Width(inline)+1+lipgloss.Width(shortcuts) <= x.r.contentWidth {
		out = append(out, place(ui.PlaceRight(x.r.contentWidth, inline, shortcuts, x.r.blockUI.Fill())))
	} else {
		for _, line := range x.wrapInline(inline) {
			out = append(out, place(line))
		}
		for _, line := range x.wrapInline(shortcuts) {
			out = append(out, ui.PlaceContentLine(line, x.r.cardWidth, x.r.contentWidth, x.r.cfg.Padding.Left, lipgloss.Right, x.r.blockUI.Fill()))
		}
	}
	if sub != "" {
		if layout != "inline" {
			out = append(out, place(""))
		}
		out = append(out, place(sub))
	}
	return out
}

// wrapInline inserts line breaks at the content width, then streams the ANSI
// output through WrapWriter so active styles are restored after every break.
func (x Activation) wrapInline(content string) []string {
	var out strings.Builder
	writer := lipgloss.NewWrapWriter(&out)
	defer writer.Close() //nolint:errcheck
	_, _ = writer.Write([]byte(ansi.Wrap(content, x.r.contentWidth, "")))
	return ui.SplitLines(out.String())
}
