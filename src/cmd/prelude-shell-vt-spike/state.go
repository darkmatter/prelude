package main

// This file is the portable part of the prototype: geometry and host-state
// transitions are pure. PTY, subprocess, and terminal code live elsewhere.

type pinMode int

const (
	pinOff pinMode = iota
	pinMotd
	pinDocs
)

func (m pinMode) label() string {
	switch m {
	case pinOff:
		return "off"
	case pinMotd:
		return "motd"
	case pinDocs:
		return "docs"
	default:
		return "?"
	}
}

func parsePinMode(target string) (pinMode, bool) {
	switch target {
	case "off":
		return pinOff, true
	case "motd":
		return pinMotd, true
	case "docs":
		return pinDocs, true
	default:
		return pinOff, false
	}
}

type hostLayout struct {
	cols      int
	height    int
	pinHeight int
	shellTop  int
	shellRows int
	statusRow int
}

func computeLayout(cols, height, configuredShellRows int, pin pinMode) hostLayout {
	cols = max(cols, 1)
	height = max(height, 3)
	configuredShellRows = max(configuredShellRows, 1)

	layout := hostLayout{
		cols:      cols,
		height:    height,
		statusRow: height - 1,
	}
	available := height - 1
	if pin == pinOff {
		layout.shellRows = available
		return layout
	}

	layout.shellRows = min(configuredShellRows, available-1)
	layout.pinHeight = available - layout.shellRows
	layout.shellTop = layout.pinHeight
	return layout
}

type panelPhase string

const (
	panelOff     panelPhase = "off"
	panelLoading panelPhase = "loading"
	panelReady   panelPhase = "ready"
	panelFailed  panelPhase = "failed"
)

type inputOwner uint8

const (
	inputShell inputOwner = iota
	inputDocs
)

func (owner inputOwner) label() string {
	if owner == inputDocs {
		return "docs"
	}
	return "shell"
}

type hostState struct {
	width            int
	height           int
	configuredRows   int
	pin              pinMode
	layout           hostLayout
	panelGeneration  uint64
	panelPhase       panelPhase
	panelSnapshot    *panelSnapshot
	panelError       string
	lastRuntimeError string
	input            inputOwner
}

func initialHostState(shellRows int) hostState {
	state := hostState{
		width:          80,
		height:         24,
		configuredRows: max(shellRows, 1),
		panelPhase:     panelOff,
	}
	state.layout = computeLayout(state.width, state.height, state.configuredRows, state.pin)
	return state
}

type stateEventKind int

const (
	resizeEvent stateEventKind = iota
	setPinEvent
	toggleInputEvent
)

type stateEvent struct {
	kind          stateEventKind
	width, height int
	pin           pinMode
}

type panelRequest struct {
	generation uint64
	mode       pinMode
	cols       int
	rows       int
}

func transition(state hostState, event stateEvent) (hostState, *panelRequest) {
	switch event.kind {
	case resizeEvent:
		state.width = max(event.width, 1)
		state.height = max(event.height, 3)
	case setPinEvent:
		state.pin = event.pin
		state.input = inputShell
	case toggleInputEvent:
		if state.pin == pinDocs && state.panelPhase == panelReady {
			if state.input == inputShell {
				state.input = inputDocs
			} else {
				state.input = inputShell
			}
		}
		return state, nil
	}

	state.layout = computeLayout(state.width, state.height, state.configuredRows, state.pin)
	if state.pin == pinOff {
		state.panelPhase = panelOff
		state.panelSnapshot = nil
		state.panelError = ""
		return state, nil
	}
	if event.kind == resizeEvent && state.pin == pinDocs {
		// A live Docs terminal receives SIGWINCH and redraws itself. Static
		// panels are recaptured because their command has already exited.
		return state, nil
	}

	state.panelGeneration++
	state.panelPhase = panelLoading
	state.panelSnapshot = nil
	state.panelError = ""
	request := &panelRequest{
		generation: state.panelGeneration,
		mode:       state.pin,
		cols:       state.layout.cols,
		rows:       state.layout.pinHeight,
	}
	return state, request
}

func acceptPanelResult(state hostState, result panelResultMsg) hostState {
	if result.generation != state.panelGeneration || result.mode != state.pin {
		return state
	}
	state.panelSnapshot = result.snapshot
	state.panelError = result.errText
	if result.errText == "" {
		state.panelPhase = panelReady
	} else {
		state.panelPhase = panelFailed
	}
	return state
}

func acceptLivePanelOutput(state hostState, generation uint64) hostState {
	if generation == state.panelGeneration && state.pin == pinDocs {
		state.panelPhase = panelReady
		state.panelError = ""
	}
	return state
}

func failLivePanel(state hostState, generation uint64, err error) hostState {
	if generation != state.panelGeneration || state.pin != pinDocs {
		return state
	}
	state.panelPhase = panelFailed
	state.panelSnapshot = nil
	state.input = inputShell
	if err != nil {
		state.panelError = err.Error()
	}
	return state
}
