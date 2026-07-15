package main

// renderFooter places status items centered in the footer row when a
// generated title is active. Without a generated title, shortcut hints
// fill the footer instead.
func (r renderer) renderFooter() string {
	if r.cfg.Title != "" {
		status := r.renderStatusItems(r.cfg.Header.Status, false)
		if status == "" {
			return ""
		}
		return r.padContentLine(status, "center")
	}
	right := r.renderShortcutItems()
	if right == "" {
		return ""
	}
	return r.padContentLine(right, "right")
}

func (r renderer) renderShortcutItems() string {
	if len(r.cfg.Shortcuts) == 0 {
		return ""
	}

	item := func(command, alias string) string {
		out := inline(r.st.muted).Bold(true).Render(command)
		if alias != "" {
			out += inline(r.st.dim).Render(" (" + alias + ")")
		}
		return out
	}

	var content string
	for i, s := range r.cfg.Shortcuts {
		if i > 0 {
			content += inline(r.st.dim).Render("  ·  ")
		}
		content += item(s.Command, s.Alias)
	}

	return content
}
