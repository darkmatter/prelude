package title

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// The wizard is an iteration of the title form: the same title/font pages,
// extended with the main prelude.* options so a new project can be configured
// in one pass. On finish it writes the config to -o (default prelude.nix) and
// a sibling title.txt beside it. The interactive UI renders on stderr.

type wizardStep uint8

const (
	stepTitle wizardStep = iota
	stepFont
	stepProject
	stepTheme
	stepProfile
	stepComponents
	stepCommands
	stepIntegration
	stepConfirm
	wizardStepCount = int(stepConfirm) + 1
)

// commandEntryPhase tracks the sub-flow of the commands step: browsing the
// list, or entering one of the three fields of a new command.
type commandEntryPhase uint8

const (
	commandList commandEntryPhase = iota
	commandName
	commandExec
	commandDescription
)

// wizardCommand is one project command destined for `prelude.commands`.
// Exec and Description may stay empty: prelude infers exec from the name and
// defaults the description.
type wizardCommand struct {
	Name        string
	Exec        string
	Description string
}

// colorProfiles mirrors the prelude.colorProfile enum declared in
// options/shared.nix; the order puts the default first.
var colorProfiles = []string{"auto", "truecolor", "ansi256"}

// componentNames drive the toggle step; the order is the emission order.
var componentNames = [4]string{"motd", "menu", "prompt", "docs"}

type wizardModel struct {
	cfg    Config
	render renderFunc
	step   wizardStep

	titleInput   textinput.Model
	projectInput textinput.Model
	// projectAuto keeps the project field following the title text until the
	// user edits the project explicitly.
	projectAuto bool

	fontIndex int
	preview   string

	themeIndex   int
	profileIndex int

	componentIndex int
	// components holds the enable toggles in componentNames order:
	// motd, menu, prompt, docs. Docs starts off because enabling it requires
	// authoring Markdown pages.
	components [4]bool

	commands       []wizardCommand
	commandPhase   commandEntryPhase
	commandCursor  int
	commandInput   textinput.Model
	pendingCommand wizardCommand

	// integrationIndex selects the emitted config shape:
	// 0 = flake-parts module, 1 = standalone prelude.lib builders.
	integrationIndex int

	width  int
	height int
	err    string

	done     bool
	canceled bool
}

// wizardResult is the pure outcome of a completed wizard run; rendering it to
// Nix is separated from the model so emission is unit-testable.
type wizardResult struct {
	Recipe       Recipe
	Project      string
	Theme        string
	ColorProfile string
	Motd         bool
	Menu         bool
	Prompt       bool
	Docs         bool
	Commands     []wizardCommand
	// FlakeParts selects the emission shape: a flake-parts module when true,
	// standalone prelude.lib builder calls when false.
	FlakeParts bool
}

func newWizard(cfg Config, recipe Recipe, render renderFunc) wizardModel {
	titleIn := textinput.New()
	titleIn.Prompt = ""
	titleIn.Placeholder = "project title"
	titleIn.SetValue(recipe.Text)
	titleIn.CursorEnd()
	titleIn.SetWidth(56)
	titleIn.SetVirtualCursor(true)
	titleIn.Focus()

	projectIn := textinput.New()
	projectIn.Prompt = ""
	projectIn.Placeholder = "project name"
	projectIn.SetWidth(56)
	projectIn.SetVirtualCursor(true)

	commandIn := textinput.New()
	commandIn.Prompt = ""
	commandIn.SetWidth(56)
	commandIn.SetVirtualCursor(true)

	fontIndex := cfg.fontIndex(recipe.Font)
	if fontIndex < 0 {
		fontIndex = cfg.fontIndex(cfg.DefaultFont)
	}
	if fontIndex < 0 {
		fontIndex = 0
	}
	themeIndex := cfg.themeIndex(cfg.DefaultTheme)
	if themeIndex < 0 {
		themeIndex = 0
	}

	return wizardModel{
		cfg:          cfg,
		render:       render,
		titleInput:   titleIn,
		projectInput: projectIn,
		commandInput: commandIn,
		projectAuto:  true,
		fontIndex:    fontIndex,
		themeIndex:   themeIndex,
		components:   [4]bool{true, true, true, false},
		width:        80,
		height:       24,
	}
}

func (m wizardModel) Init() tea.Cmd { return nil }

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.canceled = true
			return m, tea.Quit
		}
		switch m.step {
		case stepTitle:
			return m.updateTitle(msg)
		case stepFont:
			return m.updateFont(msg)
		case stepProject:
			return m.updateProject(msg)
		case stepTheme:
			return m.updateTheme(msg)
		case stepProfile:
			return m.updateProfile(msg)
		case stepComponents:
			return m.updateComponents(msg)
		case stepCommands:
			return m.updateCommands(msg)
		case stepIntegration:
			return m.updateIntegration(msg)
		case stepConfirm:
			return m.updateConfirm(msg)
		}
	}

	// Non-key messages (cursor blink) go to whichever input is focused.
	var cmd tea.Cmd
	switch m.step {
	case stepTitle:
		m.titleInput, cmd = m.titleInput.Update(msg)
	case stepProject:
		m.projectInput, cmd = m.projectInput.Update(msg)
	case stepCommands:
		if m.commandPhase != commandList {
			m.commandInput, cmd = m.commandInput.Update(msg)
		}
	}
	return m, cmd
}

func (m wizardModel) updateTitle(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.titleInput.Value())
		if text == "" {
			m.err = "title cannot be empty"
			return m, nil
		}
		m.titleInput.SetValue(text)
		m.titleInput.Blur()
		if m.projectAuto {
			m.projectInput.SetValue(text)
			m.projectInput.CursorEnd()
		}
		m.step = stepFont
		m.refreshPreview()
		return m, nil
	case "esc":
		m.canceled = true
		return m, tea.Quit
	}
	before := m.titleInput.Value()
	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(msg)
	if m.titleInput.Value() != before {
		m.err = ""
	}
	return m, cmd
}

func (m wizardModel) updateFont(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "up", "h", "k", "shift+tab":
		m.moveFont(-1)
	case "right", "down", "l", "j", "tab", "space":
		m.moveFont(1)
	case "home":
		m.fontIndex = 0
		m.refreshPreview()
	case "end":
		m.fontIndex = len(m.cfg.Fonts) - 1
		m.refreshPreview()
	case "enter":
		if m.err == "" {
			m.step = stepProject
			m.projectInput.Focus()
			m.projectInput.CursorEnd()
		}
	case "esc", "backspace":
		m.step = stepTitle
		m.err = ""
		m.titleInput.Focus()
		m.titleInput.CursorEnd()
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m wizardModel) updateProject(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		project := strings.TrimSpace(m.projectInput.Value())
		if project == "" {
			m.err = "project name cannot be empty"
			return m, nil
		}
		m.projectInput.SetValue(project)
		m.projectInput.Blur()
		m.step = stepTheme
		return m, nil
	case "esc":
		m.projectInput.Blur()
		m.step = stepFont
		m.err = ""
		return m, nil
	}
	before := m.projectInput.Value()
	var cmd tea.Cmd
	m.projectInput, cmd = m.projectInput.Update(msg)
	if m.projectInput.Value() != before {
		m.err = ""
		m.projectAuto = false
	}
	return m, cmd
}

func (m wizardModel) updateTheme(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.themeIndex = wrap(m.themeIndex-1, len(m.cfg.Themes))
	case "down", "j", "tab":
		m.themeIndex = wrap(m.themeIndex+1, len(m.cfg.Themes))
	case "home":
		m.themeIndex = 0
	case "end":
		m.themeIndex = len(m.cfg.Themes) - 1
	case "enter":
		m.step = stepProfile
	case "esc", "backspace":
		m.step = stepProject
		m.projectInput.Focus()
		m.projectInput.CursorEnd()
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m wizardModel) updateProfile(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.profileIndex = wrap(m.profileIndex-1, len(colorProfiles))
	case "down", "j", "tab":
		m.profileIndex = wrap(m.profileIndex+1, len(colorProfiles))
	case "enter":
		m.step = stepComponents
	case "esc", "backspace":
		m.step = stepTheme
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m wizardModel) updateComponents(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.componentIndex = wrap(m.componentIndex-1, len(componentNames))
	case "down", "j", "tab":
		m.componentIndex = wrap(m.componentIndex+1, len(componentNames))
	case "space", "x":
		m.components[m.componentIndex] = !m.components[m.componentIndex]
	case "enter":
		m.step = stepCommands
		m.commandPhase = commandList
	case "esc", "backspace":
		m.step = stepProfile
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m wizardModel) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.done = true
		return m, tea.Quit
	case "esc", "backspace":
		m.step = stepIntegration
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m wizardModel) updateIntegration(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab", "down", "j", "tab":
		m.integrationIndex = 1 - m.integrationIndex
	case "enter":
		m.step = stepConfirm
	case "esc", "backspace":
		m.step = stepCommands
		m.commandPhase = commandList
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

// updateCommands drives the commands step: a browsable list plus a
// three-field entry sub-flow (name, exec, description). Esc walks one level
// back at every point, mirroring the step-level navigation.
func (m wizardModel) updateCommands(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.commandPhase == commandList {
		switch msg.String() {
		case "a", "n":
			m.pendingCommand = wizardCommand{}
			m.commandPhase = commandName
			m.startCommandField("dev or db:migrate", "")
		case "d", "x":
			if len(m.commands) > 0 {
				m.commands = append(m.commands[:m.commandCursor], m.commands[m.commandCursor+1:]...)
				if m.commandCursor >= len(m.commands) && m.commandCursor > 0 {
					m.commandCursor--
				}
			}
		case "up", "k", "shift+tab":
			if len(m.commands) > 0 {
				m.commandCursor = wrap(m.commandCursor-1, len(m.commands))
			}
		case "down", "j", "tab":
			if len(m.commands) > 0 {
				m.commandCursor = wrap(m.commandCursor+1, len(m.commands))
			}
		case "enter":
			m.step = stepIntegration
		case "esc", "backspace":
			m.step = stepComponents
		case "q":
			m.canceled = true
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.String() {
	case "enter":
		return m.commitCommandField()
	case "esc":
		// Step one field back; from the name field, back to the list.
		switch m.commandPhase {
		case commandName:
			m.commandPhase = commandList
			m.commandInput.Blur()
		case commandExec:
			m.commandPhase = commandName
			m.startCommandField("dev or db:migrate", m.pendingCommand.Name)
		case commandDescription:
			m.commandPhase = commandExec
			m.startCommandField("defaults to the command name", m.pendingCommand.Exec)
		}
		m.err = ""
		return m, nil
	}
	before := m.commandInput.Value()
	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)
	if m.commandInput.Value() != before {
		m.err = ""
	}
	return m, cmd
}

// startCommandField focuses the shared entry input for the next field.
func (m *wizardModel) startCommandField(placeholder, value string) {
	m.commandInput.Placeholder = placeholder
	m.commandInput.SetValue(value)
	m.commandInput.CursorEnd()
	m.commandInput.Focus()
}

// commitCommandField validates and stores the current entry field, advancing
// to the next field or appending the finished command.
func (m wizardModel) commitCommandField() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.commandInput.Value())
	switch m.commandPhase {
	case commandName:
		if !commandKeyPattern.MatchString(value) {
			m.err = "command name must use [A-Za-z0-9_.:-] with no empty colon segments"
			return m, nil
		}
		// `list` is reserved by `menu list`; a `menu` command with a custom
		// exec would shadow the menu itself and fail the module assertion.
		if value == "list" || value == "menu" {
			m.err = fmt.Sprintf("command name %q is reserved by prelude", value)
			return m, nil
		}
		for _, existing := range m.commands {
			if existing.Name == value {
				m.err = fmt.Sprintf("command %q already exists", value)
				return m, nil
			}
		}
		m.pendingCommand.Name = value
		m.commandPhase = commandExec
		m.startCommandField("defaults to the command name", m.pendingCommand.Exec)
	case commandExec:
		m.pendingCommand.Exec = value
		m.commandPhase = commandDescription
		m.startCommandField("shown in the menu and MOTD", m.pendingCommand.Description)
	case commandDescription:
		m.pendingCommand.Description = value
		m.commands = append(m.commands, m.pendingCommand)
		m.commandCursor = len(m.commands) - 1
		m.pendingCommand = wizardCommand{}
		m.commandPhase = commandList
		m.commandInput.Blur()
	}
	return m, nil
}

func (m *wizardModel) moveFont(delta int) {
	m.fontIndex = wrap(m.fontIndex+delta, len(m.cfg.Fonts))
	m.refreshPreview()
}

func (m *wizardModel) refreshPreview() {
	preview, err := m.render(m.cfg.Fonts[m.fontIndex], m.titleInput.Value())
	if err != nil {
		m.preview = ""
		m.err = err.Error()
		return
	}
	m.preview = preview
	m.err = ""
}

func (m *wizardModel) resizeInputs() {
	width := m.width - 16
	if width > 72 {
		width = 72
	}
	if width < 20 {
		width = 20
	}
	m.titleInput.SetWidth(width)
	m.projectInput.SetWidth(width)
	m.commandInput.SetWidth(width)
}

func wrap(index, length int) int {
	return (index + length + length) % length
}

func (m wizardModel) result() wizardResult {
	return wizardResult{
		Recipe: Recipe{
			Text: strings.TrimSpace(m.titleInput.Value()),
			Font: m.cfg.Fonts[m.fontIndex].Name,
		},
		Project:      strings.TrimSpace(m.projectInput.Value()),
		Theme:        m.cfg.Themes[m.themeIndex].Name,
		ColorProfile: colorProfiles[m.profileIndex],
		Motd:         m.components[0],
		Menu:         m.components[1],
		Prompt:       m.components[2],
		Docs:         m.components[3],
		Commands:     m.commands,
		FlakeParts:   m.integrationIndex == 0,
	}
}

// --- view ---------------------------------------------------------------

func (m wizardModel) View() tea.View {
	s := newFormStyles()
	step := fmt.Sprintf("step %d/%d", int(m.step)+1, wizardStepCount)
	var body string
	switch m.step {
	case stepTitle:
		body = s.inputBody(
			"Create a Prelude title",
			"Set the text rendered in your MOTD header.  ·  "+step,
			"TITLE",
			m.titleInput,
			m.err,
			"enter continue  ·  esc cancel",
		)
	case stepFont:
		body = s.pagerBody(
			"Choose a title style",
			"Page through live previews, then confirm your selection.  ·  "+step,
			m.cfg.Fonts[m.fontIndex],
			m.fontIndex,
			len(m.cfg.Fonts),
			m.preview,
			m.err,
			m.width,
			m.height,
			"←/→ or j/k page  ·  enter choose  ·  esc edit title  ·  q cancel",
		)
	case stepProject:
		body = s.inputBody(
			"Name the project",
			"Shown in the menu header and used as the fallback wordmark.  ·  "+step,
			"PROJECT",
			m.projectInput,
			m.err,
			"enter continue  ·  esc back",
		)
	case stepTheme:
		body = s.listBody(
			"Pick a theme",
			"One palette drives every component.  ·  "+step,
			append(m.themeRows(), "", m.themePreview()),
			m.err,
			"j/k move  ·  enter choose  ·  esc back  ·  q cancel",
		)
	case stepProfile:
		rows := make([]string, len(colorProfiles))
		hints := [3]string{"detect from the terminal", "force 24-bit color", "force the 256-color palette"}
		for i, profile := range colorProfiles {
			rows[i] = listRow(s, i == m.profileIndex, fmt.Sprintf("%-11s", profile)+s.muted.Render(hints[i]))
		}
		body = s.listBody(
			"Choose the color depth",
			"auto is right unless detection guesses wrong.  ·  "+step,
			rows,
			m.err,
			"j/k move  ·  enter choose  ·  esc back  ·  q cancel",
		)
	case stepComponents:
		hints := [4]string{
			"welcome banner on shell entry",
			"interactive command picker",
			"themed starship prompt",
			"Markdown docs viewer (needs pages)",
		}
		rows := make([]string, len(componentNames))
		for i, name := range componentNames {
			mark := "[ ]"
			if m.components[i] {
				mark = "[x]"
			}
			rows[i] = listRow(s, i == m.componentIndex, mark+" "+fmt.Sprintf("%-8s", name)+s.muted.Render(hints[i]))
		}
		rows = append(rows, "", m.componentPreview())
		body = s.listBody(
			"Toggle components",
			"The card shows the highlighted component in your theme.  ·  "+step,
			rows,
			m.err,
			"j/k move  ·  space toggle  ·  enter continue  ·  esc back",
		)
	case stepCommands:
		body = m.commandsBody(s, step)
	case stepIntegration:
		options := [2]string{"flake-parts", "standalone"}
		hints := [2]string{
			"import as a flake-parts module (recommended)",
			"prelude.lib builder calls — no flake-parts",
		}
		rows := make([]string, len(options))
		for i, option := range options {
			rows[i] = listRow(s, i == m.integrationIndex, fmt.Sprintf("%-12s", option)+s.muted.Render(hints[i]))
		}
		body = s.listBody(
			"Choose the config shape",
			"How the printed config plugs into your flake.  ·  "+step,
			rows,
			m.err,
			"j/k switch  ·  enter choose  ·  esc back  ·  q cancel",
		)
	case stepConfirm:
		body = s.listBody(
			"Review",
			"The config prints to stdout; the title is written next to it.  ·  "+step,
			m.summaryRows(s),
			m.err,
			"enter print config  ·  esc back  ·  q cancel",
		)
	}
	return s.canvas(body, m.width, m.height)
}

// themeRows renders each theme as a chip strip painted on the theme's own
// background, so adjacent rows separate visually even when accent hues are
// close: the strip's background and text sample change per theme.
func (m wizardModel) themeRows() []string {
	s := newFormStyles()
	rows := make([]string, len(m.cfg.Themes))
	for i, theme := range m.cfg.Themes {
		ts := themeSample{theme: theme}
		var strip strings.Builder
		strip.WriteString(ts.seg("bg", "", " ", false))
		strip.WriteString(ts.seg("bg", "fg", "Aa ", false))
		for _, token := range []string{"accent", "accent2", "success", "warning", "error", "info"} {
			if _, ok := ts.color(token); !ok {
				continue
			}
			strip.WriteString(ts.seg("bg", token, "● ", false))
		}
		rows[i] = listRow(s, i == m.themeIndex, fmt.Sprintf("%-10s", theme.Name)+strip.String())
	}
	return rows
}

// --- theme & component sample cards ---------------------------------------

// sampleWidth fixes the preview-card width so cards render identically
// regardless of terminal size.
const sampleWidth = 44

// themeSample paints sample content with a theme's own palette tokens,
// skipping any token the palette does not define so sparse palettes stay
// renderable.
type themeSample struct{ theme Theme }

func (ts themeSample) color(token string) (color.Color, bool) {
	hex, ok := ts.theme.Palette[token]
	if !ok || hex == "" {
		return nil, false
	}
	return lipgloss.Color(hex), true
}

// seg renders text in fgToken over bgToken; unknown tokens degrade to the
// surrounding style rather than failing.
func (ts themeSample) seg(bgToken, fgToken, text string, bold bool) string {
	style := lipgloss.NewStyle().Bold(bold)
	if bg, ok := ts.color(bgToken); ok {
		style = style.Background(bg)
	}
	if fg, ok := ts.color(fgToken); ok {
		style = style.Foreground(fg)
	}
	return style.Render(text)
}

// line pads joined segments to sampleWidth on the given background layer so
// every card line forms a solid block.
func (ts themeSample) line(bgToken string, segs ...string) string {
	joined := strings.Join(segs, "")
	pad := sampleWidth - lipgloss.Width(joined)
	if pad < 0 {
		pad = 0
	}
	return joined + ts.seg(bgToken, "", strings.Repeat(" ", pad), false)
}

// sampleProject picks the text used inside preview cards: the project name
// once known, else the title text, else a stand-in.
func (m wizardModel) sampleProject() string {
	if project := strings.TrimSpace(m.projectInput.Value()); project != "" {
		return project
	}
	if title := strings.TrimSpace(m.titleInput.Value()); title != "" {
		return title
	}
	return "acme-web"
}

// themePreview renders a mini devshell card in the selected theme: one line
// per background layer (bg, surface, secondary) with the semantic colors in
// context, which separates themes far better than isolated dots.
func (m wizardModel) themePreview() string {
	ts := themeSample{theme: m.cfg.Themes[m.themeIndex]}
	project := m.sampleProject()
	return strings.Join([]string{
		ts.line("bg",
			ts.seg("bg", "accent", " ◆ "+project, true),
			ts.seg("bg", "fg", "  dev shell ready", false),
		),
		ts.line("surface",
			ts.seg("surface", "dim", " $ ", false),
			ts.seg("surface", "accent", "check", false),
			ts.seg("surface", "muted", " ─ build + render smoke tests", false),
		),
		ts.line("secondary",
			ts.seg("secondary", "success", " ● ok", false),
			ts.seg("secondary", "warning", "  ● warn", false),
			ts.seg("secondary", "error", "  ● fail", false),
			ts.seg("secondary", "info", "  ● info", false),
		),
	}, "\n")
}

// componentPreview mocks the highlighted component in the already-selected
// theme, so the toggle decision is made against what the component actually
// looks like rather than a one-line description.
func (m wizardModel) componentPreview() string {
	ts := themeSample{theme: m.cfg.Themes[m.themeIndex]}
	project := m.sampleProject()
	switch componentNames[m.componentIndex] {
	case "motd":
		return strings.Join([]string{
			ts.line("bg",
				ts.seg("bg", "accent", " "+project, true),
				ts.seg("bg", "fg", "  Dev Shell Activated", false),
			),
			ts.line("bg",
				ts.seg("bg", "muted", "   your environment is ready", false),
			),
			ts.line("bg",
				ts.seg("bg", "dim", " $ ", false),
				ts.seg("bg", "accent", "check", false),
				ts.seg("bg", "muted", " ····· build + render smoke tests", false),
			),
		}, "\n")
	case "menu":
		return strings.Join([]string{
			ts.line("surface",
				ts.seg("surface", "accent2", " › ", true),
				ts.seg("surface", "fg", "che", false),
				ts.seg("surface", "dim", "▌", false),
			),
			ts.line("surface",
				ts.seg("surface", "accent", " ▸ check", true),
				ts.seg("surface", "muted", "   build + render smoke tests", false),
			),
			ts.line("surface",
				ts.seg("surface", "fg", "   fmt", false),
				ts.seg("surface", "muted", "     format nix sources", false),
			),
		}, "\n")
	case "prompt":
		return strings.Join([]string{
			ts.line("bg", ""),
			ts.line("bg",
				ts.seg("secondary", "fg", " "+project+" ", false),
				ts.seg("bg", "accent", "  main", false),
			),
			ts.line("bg",
				ts.seg("bg", "accent2", " ❯ ", true),
				ts.seg("bg", "fg", "menu", false),
			),
		}, "\n")
	case "docs":
		return strings.Join([]string{
			ts.line("surface",
				ts.seg("surface", "accent", " 1 Welcome", true),
				ts.seg("surface", "muted", "   2 Commands", false),
			),
			ts.line("surface",
				ts.seg("surface", "accent2", " # Getting started", true),
			),
			ts.line("surface",
				ts.seg("surface", "fg", " Each Markdown file is one page.", false),
			),
		}, "\n")
	}
	return ""
}

// commandsBody renders the commands step: the entry sub-form while a command
// is being added, otherwise the browsable list of collected commands.
func (m wizardModel) commandsBody(s formStyles, step string) string {
	if m.commandPhase != commandList {
		labels := map[commandEntryPhase]string{
			commandName:        "NAME",
			commandExec:        "EXEC (shell text, optional)",
			commandDescription: "DESCRIPTION (optional)",
		}
		subtitle := "Add a command to the catalogue.  ·  " + step
		if m.pendingCommand.Name != "" {
			subtitle = m.pendingCommand.Name + "  ·  " + subtitle
		}
		return s.inputBody(
			"Add a command",
			subtitle,
			labels[m.commandPhase],
			m.commandInput,
			m.err,
			"enter next field  ·  esc previous",
		)
	}

	var rows []string
	if len(m.commands) == 0 {
		rows = []string{s.dim.Render("  (no commands yet — press a to add one)")}
	} else {
		rows = make([]string, len(m.commands))
		for i, command := range m.commands {
			exec := command.Exec
			if exec == "" {
				exec = command.Name
			}
			detail := s.muted.Render("  " + exec)
			if command.Description != "" {
				detail += s.dim.Render("  — " + command.Description)
			}
			rows[i] = listRow(s, i == m.commandCursor, fmt.Sprintf("%-12s", command.Name)+detail)
		}
	}
	return s.listBody(
		"Project commands",
		"Shown in the menu; the first three become MOTD next steps.  ·  "+step,
		rows,
		m.err,
		"a add  ·  d delete  ·  j/k move  ·  enter continue  ·  esc back",
	)
}

func (m wizardModel) summaryRows(s formStyles) []string {
	result := m.result()
	onOff := func(enabled bool) string {
		if enabled {
			return "on"
		}
		return "off"
	}
	line := func(label, value string) string {
		return s.dim.Render(fmt.Sprintf("%-10s", label)) + s.base.Render(value)
	}
	commands := "none"
	if len(result.Commands) > 0 {
		names := make([]string, len(result.Commands))
		for i, command := range result.Commands {
			names[i] = command.Name
		}
		commands = strings.Join(names, ", ")
	}
	return []string{
		line("title", result.Recipe.Text+"  ("+result.Recipe.Font+")"),
		line("project", result.Project),
		line("theme", result.Theme),
		line("colors", result.ColorProfile),
		line("commands", commands),
		line("output", map[bool]string{true: "flake-parts module", false: "standalone builders"}[result.FlakeParts]),
		line("motd", onOff(result.Motd)),
		line("menu", onOff(result.Menu)),
		line("prompt", onOff(result.Prompt)),
		line("docs", onOff(result.Docs)),
	}
}

// listBody renders a heading plus prebuilt rows; rows carry their own cursor
// and styling so theme swatches and toggles stay flexible.
func (s formStyles) listBody(heading, subtitle string, rows []string, errMsg, footer string) string {
	parts := []string{
		s.title.Render(heading),
		s.muted.Render(subtitle),
		"",
	}
	parts = append(parts, rows...)
	if errMsg != "" {
		parts = append(parts, "", s.err.Render(errMsg))
	}
	parts = append(parts, "", s.dim.Render(footer))
	return s.panel.Render(strings.Join(parts, "\n"))
}

func listRow(s formStyles, selected bool, content string) string {
	if selected {
		return s.title.Render("▸ ") + content
	}
	return s.base.Render("  ") + content
}
