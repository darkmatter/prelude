package main

import uv "github.com/charmbracelet/ultraviolet"

type placement uint8

const (
	placementFloating placement = iota
	placementLeft
	placementRight
	placementTop
	placementBottom
)

func (p placement) String() string {
	switch p {
	case placementLeft:
		return "left"
	case placementRight:
		return "right"
	case placementTop:
		return "top"
	case placementBottom:
		return "bottom"
	default:
		return "floating"
	}
}

func (p placement) next() placement {
	switch p {
	case placementFloating:
		return placementLeft
	case placementLeft:
		return placementRight
	case placementRight:
		return placementTop
	case placementTop:
		return placementBottom
	default:
		return placementFloating
	}
}

func (p placement) prev() placement {
	switch p {
	case placementFloating:
		return placementBottom
	case placementRight:
		return placementLeft
	case placementTop:
		return placementRight
	case placementBottom:
		return placementTop
	default:
		return placementFloating
	}
}

type workspaceLayout struct {
	Shell       uv.Rectangle
	Panel       uv.Rectangle
	Body        uv.Rectangle
	Completion  uv.Rectangle
	ShellScroll int // display-only native row offset; never a PTY size change
	FooterY     int
}

// computeLayout keeps the footer outside both panes. Body fills the borderless
// Panel, with no host title strip or inset. At tiny sizes, empty Panel and Body
// mean suspension: the host retains the pane and pauses its input/resizing until
// Body is nonempty again.
func computeLayout(cols, rows int, where placement, panelOpen bool) workspaceLayout {
	cols, rows = max(cols, 1), max(rows, 1)
	contentRows := max(rows-1, 1) // Same usable height as shellRows.
	layout := workspaceLayout{
		Shell:   uv.Rect(0, 0, cols, contentRows),
		FooterY: -1,
	}
	if rows > 1 {
		layout.FooterY = rows - 1
	}
	if !panelOpen {
		return layout
	}

	switch where {
	case placementLeft, placementRight:
		if cols < 4 || contentRows < 3 {
			return layout
		}
		width := max(cols/2, 3)
		layout.Panel = uv.Rect(0, 0, width, contentRows)
		layout.Shell = uv.Rect(width, 0, cols-width, contentRows)
		if where == placementRight {
			layout.Panel = uv.Rect(cols-width, 0, width, contentRows)
			layout.Shell = uv.Rect(0, 0, cols-width, contentRows)
		}
	case placementTop, placementBottom:
		if cols < 3 || contentRows < 4 {
			return layout
		}
		height := max(contentRows/2, 3)
		layout.Panel = uv.Rect(0, 0, cols, height)
		layout.Shell = uv.Rect(0, height, cols, contentRows-height)
		if where == placementBottom {
			layout.Panel = uv.Rect(0, contentRows-height, cols, height)
			layout.Shell = uv.Rect(0, 0, cols, contentRows-height)
		}
	default:
		if cols < 3 || contentRows < 3 {
			return layout
		}
		// Roughly 80%, capped at 100x30, with an outside margin on each
		// edge whenever the minimum 3x3 frame leaves room for one.
		width := min(100, max(3, cols-cols/5), max(3, cols-2))
		height := min(30, max(3, contentRows-contentRows/5), max(3, contentRows-2))
		layout.Panel = uv.Rect((cols-width)/2, (contentRows-height)/2, width, height)
	}
	layout.Body = layout.Panel
	return layout
}
