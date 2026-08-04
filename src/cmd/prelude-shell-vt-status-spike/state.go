package main

// The status-only host has no panel state and no line-edit mirror. ble.sh owns
// completion descriptions, ghost text, and syntax validation inside the child
// PTY; the host owns a viewport into the shell's retained scrollback plus the
// thin 2-row chrome (key hints + live ● ready / ● error status).
type hostLayout struct {
	cols       int
	height     int
	shellRows  int
	statusTop  int
	statusRows int
}

func computeLayout(cols, height int) hostLayout {
	cols = max(cols, 1)
	height = max(height, 1)
	statusRows := min(2, max(height-1, 0))
	shellRows := height - statusRows
	return hostLayout{
		cols:       cols,
		height:     height,
		shellRows:  shellRows,
		statusTop:  shellRows,
		statusRows: statusRows,
	}
}

type hostState struct {
	layout           hostLayout
	scroll           int
	scrollbackLimit  int
	lastRuntimeError string
}

func initialHostState(cols, rows, scrollbackLimit int) hostState {
	return hostState{
		layout:          computeLayout(cols, rows),
		scrollbackLimit: max(scrollbackLimit, 1),
	}
}

func (state hostState) resize(cols, rows int) hostState {
	state.layout = computeLayout(cols, rows)
	return state
}

func (state hostState) scrollBy(delta, history int) hostState {
	state.scroll = clamp(state.scroll+delta, 0, max(history, 0))
	return state
}

func (state hostState) followTail() hostState {
	state.scroll = 0
	return state
}

func clamp(value, low, high int) int {
	return min(max(value, low), high)
}
