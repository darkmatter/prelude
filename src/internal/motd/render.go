package motd

// render produces the full MOTD for a terminal of the given width.
func render(cfg Config, terminalWidth, terminalHeight int, runtime Runtime) string {
	return (MOTDView{r: newRenderer(cfg, terminalWidth, terminalHeight, runtime)}).Render()
}
