package motd

// render produces the full MOTD for a terminal of the given width.
// Pure: no Runtime, no Cache I/O — cfg must already carry applied live fields.
func render(cfg Config, terminalWidth, terminalHeight int) string {
	return (MOTDView{r: newRenderer(cfg, terminalWidth, terminalHeight)}).Render()
}
