package motd

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

// Env is a React-style, one-component-per-file presentation of the motd env
// section. It renders tool versions as one flowing row of chips. The component
// is stateless and uses the resolved renderer context for config, styles,
// dimensions, and runtime probes.
type Env struct {
	r renderer
}

// Render builds tool versions as one flowing row of chips.
func (x Env) Render() []string {
	var row strings.Builder
	for _, item := range x.r.cfg.Env {
		if rendered, ok := x.renderEnvItem(item); ok {
			row.WriteString(rendered)
		}
	}

	if strings.TrimSpace(ansi.Strip(row.String())) == "" {
		return nil
	}
	return x.WrapAndFill(row.String(), x.r.contentWidth)
}

func (x Env) renderEnvItem(item EnvItem) (string, bool) {
	value := item.Value
	if item.Probe != "" {
		probed, err := x.r.runtime.Probe(item.Probe)
		if err != nil || probed == "" {
			return "", false
		}
		value = probed
	}
	return ui.Inline(x.r.st.dim).Render(item.Label+" ") +
		ui.Inline(x.r.st.fgBold).Render(value+"   "), true
}

// WrapAndFill wraps an already-styled string on display width, then pads each
// line. Exported so Description can call it across files within the package.
func (x Env) WrapAndFill(value string, width int) []string {
	s := x.r.st.blockFill.Width(width).MaxWidth(width)
	var bl ui.Block
	for _, line := range strings.Split(lipgloss.Wrap(value, width, ""), "\n") {
		bl.Write(s.Render(line))
	}
	return ui.SplitLines(bl.String())
}
