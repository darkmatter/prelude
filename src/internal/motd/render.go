package motd

// render produces the full MOTD for a terminal of the given width.
// Pure: no Runtime, no Cache I/O — model must already carry applied live fields.
func render(model PaintModel) string {
	return (MOTDView{r: newRenderer(model)}).Render()
}
