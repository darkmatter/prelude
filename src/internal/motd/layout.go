package motd

import "prelude/pkg/ui"

// Layout constants encode hard-coded column geometry — not configuration.
const (
	minimumCardWidth = 10
	headerRightPad   = 2 // keep status off the header edge
)

// renderer is immutable render context for one MOTD pass. It carries the
// resolved PaintModel, palette styles, and geometry; named UI components
// receive this context and own presentation in their own files.
type renderer struct {
	model            PaintModel
	st               styles
	blockUI          ui.Context
	headerUI         ui.Context
	terminalWidth    int
	terminalHeight   int
	cardWidth        int
	contentWidth     int
	horizontalOffset int
}

func newRenderer(model PaintModel) renderer {
	st := newStyles(model)
	r := renderer{
		model:            model,
		st:               st,
		blockUI:          ui.NewContext(model.Config.Palette, st.blockBg, st.blockTransparent),
		headerUI:         ui.NewContext(model.Config.Palette, st.headerBg, st.headerTransparent),
		terminalWidth:    model.TerminalWidth,
		terminalHeight:   model.TerminalHeight,
		cardWidth:        model.CardWidth,
		contentWidth:     model.ContentWidth,
		horizontalOffset: model.HorizontalOffset,
	}
	return r
}
