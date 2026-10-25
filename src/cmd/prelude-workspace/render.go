package main

import (
	"image"

	"strings"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/ultraviolet/screen"
	"github.com/charmbracelet/x/ansi"
)

func drawStyledText(buffer *uv.ScreenBuffer, text string, x, y, width int, style uv.Style) {
	if width <= 0 {
		return
	}
	ctx := screen.NewContext(buffer)
	ctx.SetStyle(style)
	ctx.DrawString(ansi.Truncate(displayText(text), width, ""), x, y)
}

func drawFrame(buffer *uv.ScreenBuffer, frame terminalFrame, area uv.Rectangle) {
	drawFrameRows(buffer, frame, area, 0)
}

func drawFrameRows(buffer *uv.ScreenBuffer, frame terminalFrame, area uv.Rectangle, rowOffset int) {
	// Default-colored spaces mask underlying shell text without imposing a
	// backdrop. Explicit child colors (including selection bars) stay untouched.
	fill := uv.EmptyCell
	buffer.FillArea(&fill, area)
	for y := 0; y < min(area.Dy(), frame.Rows-rowOffset); y++ {
		for x := 0; x < min(area.Dx(), frame.Cols); {
			index := (y+rowOffset)*frame.Cols + x
			if index >= len(frame.Cells) {
				break
			}
			cell := frame.Cells[index]
			if cell.Width <= 0 || x+cell.Width > area.Dx() {
				buffer.SetCell(area.Min.X+x, area.Min.Y+y, &fill)
				x++
				continue
			}

			// SetCell writes wide-cell continuations itself. Copying the native
			// zero-width continuation afterward would erase its grapheme.
			buffer.SetCell(area.Min.X+x, area.Min.Y+y, &cell)
			x += cell.Width
		}
	}
}

func (h *host) drawPane(buffer *uv.ScreenBuffer, layout workspaceLayout) {
	body := layout.Body
	if body.Empty() {
		return
	}
	frame := terminalFrame{}
	if h.pane != nil {
		frame = h.pane.frame
	}
	drawFrame(buffer, frame, body)
	// Output/exit errors preserve the child's final image. Only startup failure
	// lacks an image, so show its error in the body as well as the notice.
	if h.pane == nil && h.paneError != nil {
		text := "error: " + displayText(h.paneError.Error()) + "\nAlt+x/d retries; Ctrl+P then c closes"
		lines := strings.Split(ansi.Hardwrap(text, body.Dx(), true), "\n")
		for y, line := range lines {
			if y >= body.Dy() {
				break
			}
			drawStyledText(buffer, line, body.Min.X, body.Min.Y+y, body.Dx(), uv.Style{Fg: h.palette.Error})
		}
	}
}

// View paints a disposable frame; it never edits either native snapshot. Hiding
// the pane therefore reveals the current shell, not an old saved underlay.
func (h *host) View() tea.View {
	layout := h.layout()
	buffer := uv.NewScreenBuffer(h.cols, h.rows)
	buffer.Fill(&uv.EmptyCell)
	drawFrame(&buffer, h.frame, layout.Shell)
	h.drawPane(&buffer, layout)
	if layout.FooterY >= 0 {
		h.drawFooter(&buffer, layout.FooterY)
	}

	// Bubble Tea reparses this ANSI string rather than consuming our cells.
	// Native/outer width agreement remains a separate compatibility concern.
	view := tea.NewView(buffer.Render())
	view.AltScreen, view.ReportFocus = true, true
	view.WindowTitle = displayText(h.frame.Title)
	// Capture the wheel even when neither child tracks the mouse. Left to the
	// outer terminal, a wheel tick becomes Up/Down on the alternate screen,
	// which Bash reads as history. Cell motion covers clicks, drags, and the
	// wheel; a child that asked for any-motion still gets the richer mode.
	// The outer terminal no longer owns highlight-to-copy; Shift+drag selects.
	view.MouseMode = tea.MouseModeCellMotion
	if (!layout.Shell.Empty() && h.frame.MouseTracking) ||
		(!layout.Body.Empty() && h.currentPane(h.pane) && h.pane.frame.MouseTracking) {
		view.MouseMode = tea.MouseModeAllMotion
	}
	frame, cursorArea := h.frame, layout.Shell
	paneFocused := h.paneFocused()
	if paneFocused {
		frame, cursorArea = h.pane.frame, layout.Body
		if frame.Title != "" {
			view.WindowTitle = displayText(frame.Title)
		}
	}
	x, y := frame.CursorX+cursorArea.Min.X, frame.CursorY+cursorArea.Min.Y
	point := image.Pt(x, y)
	// The native cursor stays on the live screen while the viewport shows
	// history, so drawing it would land on unrelated scrollback content.
	if h.terminalFocused && !frame.Scrolled && frame.CursorVisible && point.In(cursorArea) &&
		(paneFocused || !point.In(layout.Panel)) {
		view.Cursor = tea.NewCursor(x, y)
		switch frame.CursorStyle {
		case 1:
			view.Cursor.Shape = tea.CursorBar
		case 2:
			view.Cursor.Shape = tea.CursorUnderline
		}
		view.Cursor.Blink = frame.CursorBlink
	}
	return view
}
