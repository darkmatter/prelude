package manual

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// htmlCenterBlock matches the GitHub README hero div (images + tagline + chips).
var htmlCenterBlock = regexp.MustCompile(`(?is)<div\s+align="center">.*?</div>\s*`)

var htmlStrong = regexp.MustCompile(`(?is)<strong>\s*(.*?)\s*</strong>`)
var htmlSub = regexp.MustCompile(`(?is)<sub>\s*(.*?)\s*</sub>`)
var htmlTag = regexp.MustCompile(`(?is)<[^>]+>`)

// readmeIntro holds prose extracted from the root README HTML hero so the TUI
// can style it without relying on glamour's weak HTML support.
type readmeIntro struct {
	Tagline string
	Chips   string
	Body    string
}

func parseRootReadme(source string) readmeIntro {
	intro := readmeIntro{Body: source}
	loc := htmlCenterBlock.FindStringIndex(source)
	if loc == nil {
		return intro
	}
	block := source[loc[0]:loc[1]]
	if m := htmlStrong.FindStringSubmatch(block); len(m) > 1 {
		intro.Tagline = collapseWS(stripTags(m[1]))
	}
	if m := htmlSub.FindStringSubmatch(block); len(m) > 1 {
		intro.Chips = collapseWS(stripTags(m[1]))
	}
	intro.Body = strings.TrimLeft(source[loc[1]:], "\n")
	return intro
}

func stripTags(s string) string {
	return htmlTag.ReplaceAllString(s, "")
}

func collapseWS(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// renderRootReadme styles the project README: an optional FIGlet hero wordmark
// (baked at Nix build time), tagline/chips from the HTML center block, then
// glamour on the remaining markdown body. The hero is rendered only when it
// fits the text width; otherwise the project name falls back to a bold heading.
func (v Viewer) renderRootReadme(leaf *NavNode, textWidth int) []string {
	textWidth = max(textWidth, 24)
	lines := []string{v.styles.blankLine(textWidth)}

	name := strings.TrimSpace(v.document.Project)
	if name == "" {
		name = leaf.Title
	}

	// Build-time FIGlet hero: render when baked and when its widest line fits
	// the text width (with a 2-cell left indent). Long project names fall back
	// to the bold plain-name heading below.
	hero := strings.TrimRight(v.document.Hero, "\n")
	heroFits := false
	if hero != "" {
		maxW := 0
		for _, row := range strings.Split(hero, "\n") {
			if w := ansi.StringWidth(row); w > maxW {
				maxW = w
			}
		}
		heroFits = maxW <= textWidth-2
	}
	if heroFits {
		for _, row := range strings.Split(hero, "\n") {
			painted := v.styles.indent(2) + v.styles.onBody(Accent2, true).Render(row)
			lines = append(lines, v.styles.fillLine(painted, textWidth))
		}
		lines = append(lines, v.styles.blankLine(textWidth))
	} else if name != "" {
		title := v.styles.indent(2) + v.styles.onBody(Accent2, true).Render(name)
		lines = append(lines, v.styles.fillLine(title, textWidth))
		lines = append(lines, v.styles.blankLine(textWidth))
	}

	intro := parseRootReadme(leaf.Markdown)
	indent := 2
	if intro.Tagline != "" {
		wrapped := ansi.Wordwrap(intro.Tagline, max(textWidth-indent*2, 8), "")
		for _, row := range strings.Split(wrapped, "\n") {
			painted := v.styles.indent(indent) + v.styles.onBody(Accent, true).Render(row)
			lines = append(lines, v.styles.fillLine(painted, textWidth))
		}
		lines = append(lines, v.styles.blankLine(textWidth))
	}
	if intro.Chips != "" {
		// Keep wrap indent aligned with the first chip line.
		chipIndent := indent + 2
		wrapped := ansi.Wordwrap(intro.Chips, max(textWidth-chipIndent*2, 8), " ")
		for _, row := range strings.Split(wrapped, "\n") {
			painted := v.styles.indent(chipIndent) + v.styles.onBody(Muted, false).Render(row)
			lines = append(lines, v.styles.fillLine(painted, textWidth))
		}
		lines = append(lines, v.styles.blankLine(textWidth))
	}

	if strings.TrimSpace(intro.Body) != "" {
		for _, line := range v.renderMarkdownBody(intro.Body, textWidth) {
			lines = append(lines, v.styles.fillLine(line, textWidth))
		}
	}
	return lines
}
