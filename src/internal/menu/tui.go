package menu

import (
	"log"

	tea "charm.land/bubbletea/v2"
)

type mode int

const (
	modeList mode = iota
	modeArgs
)

// chip is one selectable suggested value in argument-entry mode.
type chip struct {
	arg   Arg
	label string
	value string
}

type model struct {
	cfg *Config
	st  styles

	flat    []Task
	prompt  Prompt // filter query (list) / arg string (args) — owns its textinput
	matches []int  // indices into flat, group order preserved
	sel     int    // index into matches

	expanded bool
	mode     mode

	list   *ListView     // list body sub-model: owns scroll offset + cached rows
	args   *ArgsView     // arg-entry sub-model: owns chips/chipFocus/argErr/argTask
	title  titleBar      // chrome title bar (presentational)
	status statusBar     // chrome status footer (presentational)
	frame  Frame         // rounded panel border decorator (presentational)

	width, height int
	execCmd       string // consumed by main after the TUI quits
	hasExecCmd    bool   // distinguishes a valid empty command from no selection
}

func newModel(cfg *Config, st styles, argTask *Task) model {
	m := model{
		cfg:    cfg,
		st:     st,
		flat:   cfg.flatten(),
		prompt: newPrompt(st, cfg.Project, cfg.Placeholder, 80),
		list:   newListView(st, 80),
		args:   newArgsView(st),
		title:  titleBar{st: st},
		status: statusBar{st: st},
		frame:  Frame{st: st},
		width:  80,
		height: 24,
	}
	m.resizeChrome()
	m.filter()
	m.syncList()
	if argTask != nil {
		m.enterArgMode(*argTask)
	}
	return m
}

func (m model) Init() tea.Cmd { return m.prompt.Init() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resizeChrome()
		m.syncList()
		return m, nil

	case tea.KeyPressMsg:
		if debugLog {
			log.Printf("key=%q mode=%d sel=%d matches=%d", msg.String(), m.mode, m.sel, len(m.matches))
		}
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeArgs:
			return m.updateArgs(msg)
		}
		return m.updateList(msg)

	case tea.MouseClickMsg:
		return m, nil

	case tea.MouseWheelMsg:
		if m.mode == modeList {
			var cmd tea.Cmd
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.prompt, cmd = m.prompt.Update(msg)
	return m, cmd
}
