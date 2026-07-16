package menu

import (
	"fmt"

	"charm.land/lipgloss/v2"
)

// --- list view ---------------------------------------------------------------

// Fixed panel section heights in rows. title(3)+prompt(1)+frame(1)=5 is the
// chrome above the list; chromeRows (view.go) adds the status(3) below. The
// list section is variable (listHeight) and fills the gap between them.
const (
	titleRows  = 3 // chrome.titleRow: half-pad + title + half-pad
	promptRows = 1 // prompt.View: the filter/arg input row
	frameRows  = 1 // frameTop
)

func (m model) listHeight() int {
	return max(min(m.cfg.Height, m.height-chromeRows), 4)
}

func (m model) viewList() string {
	title := fmt.Sprintf("%s — command menu — %d of %d", m.cfg.Project, len(m.matches), len(m.flat))

	// The panel is a vertical stack of section layers. The compositor places
	// each layer by its Y offset and renders the union to one string — the
	// panel is declared as a layer tree, not assembled from an ordered []string
	// joined by newlines. Section renderers stay self-contained: each returns
	// its own content; the compositor owns placement. The list is one scroll
	// layer; its visible rows are flattened into that layer's content.
	//
	// The list body is rendered by ListView (listview.go), a self-contained
	// bubbletea sub-model that owns the scroll offset and caches the visible
	// rows at Sync time. View() is a pure return of that cache; Height() gives
	// the row count so the status layer can be placed below.
	listY := titleRows + promptRows + frameRows
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(m.title.View(title)).Y(0),
		lipgloss.NewLayer(m.prompt.View()).Y(titleRows),
		lipgloss.NewLayer(m.frame.Top()).Y(titleRows+promptRows),
		lipgloss.NewLayer(m.list.View()).Y(listY),
		lipgloss.NewLayer(m.status.View([][2]string{
			{"↑ ↓", "navigate"}, {"⇥", "details"}, {"↵", "run"}, {"esc", "clear"},
		}, "● ready")).Y(listY+m.list.Height()),
	).Render()
}
