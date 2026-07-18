package ui

// Link renders a clickable OSC 8 terminal hyperlink using the context's accent
// color. Terminals without hyperlink support still display the underlined label.
type Link struct {
	Context Context
	Label   string
	URL     string
}

func (l Link) Render() string {
	if l.Label == "" {
		return ""
	}

	style := l.Context.Accent().Underline(true)
	if l.URL != "" {
		style = style.Hyperlink(l.URL)
	}
	return style.Render(l.Label)
}
