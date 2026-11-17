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

// listFrame is one open subcommand picker: the parent whose children the list
// shows, and a snapshot of the list it replaced, so closing it restores the
// exact filtered position the user left.
type listFrame struct {
	parent            Task
	flat              []Task
	matches           []int
	sel               int
	expanded          bool
	promptValue       string
	promptCtx         string
	promptPlaceholder string
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

	// pickers holds one frame per open subcommand picker, outermost first;
	// the last one's parent owns the list currently shown. Empty at the root.
	pickers []listFrame

	list   *ListView // list body sub-model: owns scroll offset + cached rows
	args   *ArgsView // arg-entry sub-model: owns chips/chipFocus/argErr/argTask
	title  titleBar  // chrome title bar (presentational)
	status statusBar // chrome status footer (presentational)
	frame  Frame     // rounded panel border decorator (presentational)

	layout Layout // terminal and panel geometry (single source of truth)

	promptCtx         string // context label left of the caret
	promptPlaceholder string // current placeholder text

	// chosen is what the picker resolved, consumed after the TUI quits; nil
	// means the user left without choosing (a valid empty command is non-nil).
	chosen *Selection
	xtab   *xtabProto // x-tab prototype sync (xtab_prototype.go); nil unless opted in
}

func newModel(cfg *Config, st styles, argTask *Task) model {
	m := model{
		cfg:               cfg,
		st:                st,
		flat:              cfg.flatten(),
		prompt:            newPrompt(st, 80),
		list:              newListView(st, 80),
		args:              newArgsView(st),
		title:             titleBar{st: st},
		status:            statusBar{st: st},
		frame:             Frame{st: st},
		promptCtx:         "~/" + cfg.Project,
		promptPlaceholder: cfg.Placeholder,
	}
	m.applyLayout(80, 24)
	m.filter()
	m.xtab = newXtabProtoFromEnv()
	m.syncList()
	if argTask != nil {
		m.enterArgMode(*argTask)
	}
	return m
}

func (m model) Init() tea.Cmd { return m.protoInit() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.applyLayout(msg.Width, msg.Height)
		m.syncList()
		return m, nil

	case tea.KeyPressMsg:
		if debugLog {
			log.Printf("key=%q mode=%d sel=%d matches=%d", msg.String(), m.mode, m.sel, len(m.matches))
		}
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.xtab != nil && xtabProtoTriggerKey(msg) {
			return m.protoHandleTrigger()
		}
		prevMode, prevPickers := m.mode, len(m.pickers)
		var cmd tea.Cmd
		if m.mode == modeArgs {
			m, cmd = m.updateArgs(msg)
		} else {
			m, cmd = m.updateList(msg)
		}
		return m.protoPublish(prevMode, prevPickers, cmd)
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
	prevMode, prevPickers := m.mode, len(m.pickers)
	m.prompt, cmd = m.prompt.Update(msg)
	return m.protoPublish(prevMode, prevPickers, cmd)
}
