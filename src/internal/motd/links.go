package motd

import "prelude/pkg/ui"

// Links renders configured terminal hyperlinks beneath the description.
type Links struct {
	r renderer
}

func (x Links) Render() []string {
	if len(x.r.cfg.Links) == 0 {
		return nil
	}

	surface := ui.Surface{Context: x.r.blockUI, Width: x.r.contentWidth}
	var lines []string
	for _, link := range x.r.cfg.Links {
		for _, labelLine := range ui.WrapText(link.Label, x.r.contentWidth) {
			rendered := (ui.Link{
				Context: x.r.blockUI,
				Label:   labelLine,
				URL:     link.URL,
			}).Render()
			if rendered != "" {
				lines = append(lines, surface.Fill(rendered))
			}
		}
	}
	return lines
}
