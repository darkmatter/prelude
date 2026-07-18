package title

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"prelude/pkg/ui"
)

type chooserStage uint8

const (
	stageTitle chooserStage = iota
	stageStyle
)

type chooserModel struct {
	cfg      Config
	input    textinput.Model
	stage    chooserStage
	selected int
	preview  string
	err      string
	width    int
	height   int
	done     bool
	canceled bool
	render   renderFunc
}

func newChooser(cfg Config, recipe Recipe, render renderFunc) chooserModel {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "project title"
	in.SetValue(recipe.Text)
	in.CursorEnd()
	in.SetWidth(56)
	in.SetVirtualCursor(true)
	in.Focus()

	selected := cfg.fontIndex(recipe.Font)
	if selected < 0 {
		selected = cfg.fontIndex(cfg.DefaultFont)
	}
	if selected < 0 {
		selected = 0
	}

	return chooserModel{
		cfg:      cfg,
		input:    in,
		selected: selected,
		width:    80,
		height:   24,
		render:   render,
	}
}

func (m chooserModel) Init() tea.Cmd { return nil }

func (m chooserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInput()
		return m, nil
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			m.canceled = true
			return m, tea.Quit
		}
		if m.stage == stageTitle {
			return m.updateTitle(msg)
		}
		return m.updateStyle(msg)
	}

	if m.stage == stageTitle {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m chooserModel) updateTitle(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			m.err = "title cannot be empty"
			return m, nil
		}
		m.input.SetValue(text)
		m.input.Blur()
		m.stage = stageStyle
		m.refreshPreview()
		return m, nil
	case "esc":
		m.canceled = true
		return m, tea.Quit
	}

	before := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != before {
		m.err = ""
	}
	return m, cmd
}

func (m chooserModel) updateStyle(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "left", "up", "h", "k", "shift+tab":
		m.move(-1)
	case "right", "down", "l", "j", "tab", "space":
		m.move(1)
	case "home":
		m.selected = 0
		m.refreshPreview()
	case "end":
		m.selected = len(m.cfg.Fonts) - 1
		m.refreshPreview()
	case "enter":
		if m.err == "" {
			m.done = true
			return m, tea.Quit
		}
	case "esc", "backspace":
		m.stage = stageTitle
		m.err = ""
		m.input.Focus()
		m.input.CursorEnd()
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *chooserModel) move(delta int) {
	m.selected = (m.selected + delta + len(m.cfg.Fonts)) % len(m.cfg.Fonts)
	m.refreshPreview()
}

func (m *chooserModel) refreshPreview() {
	preview, err := m.render(m.cfg.Fonts[m.selected], m.input.Value())
	if err != nil {
		m.preview = ""
		m.err = err.Error()
		return
	}
	m.preview = preview
	m.err = ""
}

func (m *chooserModel) resizeInput() {
	width := m.width - 16
	if width > 72 {
		width = 72
	}
	if width < 20 {
		width = 20
	}
	m.input.SetWidth(width)
}

func (m chooserModel) selectedRecipe() Recipe {
	return Recipe{Text: strings.TrimSpace(m.input.Value()), Font: m.cfg.Fonts[m.selected].Name}
}

func (m chooserModel) View() tea.View {
	s := newFormStyles()
	var body string
	if m.stage == stageTitle {
		body = s.inputBody(
			"Create a Prelude title",
			"Set the text rendered in your MOTD header.",
			"TITLE",
			m.input,
			m.err,
			"enter continue  ·  esc cancel",
		)
	} else {
		body = s.pagerBody(
			"Choose a title style",
			"Page through live previews, then confirm your selection.",
			m.cfg.Fonts[m.selected],
			m.selected,
			len(m.cfg.Fonts),
			m.preview,
			m.err,
			m.width,
			m.height,
			"←/→ or j/k page  ·  enter choose  ·  esc edit title  ·  q cancel",
		)
	}
	return s.canvas(body, m.width, m.height)
}

// formPalette is the fixed chrome shared by the chooser and wizard screens.
// These tools run before a project theme exists, so the styling is
// intentionally self-contained rather than palette-driven.
var (
	formBg     = lipgloss.Color("#0e0b13")
	formFg     = lipgloss.Color("#d6d2df")
	formMuted  = lipgloss.Color("#8787af")
	formDim    = lipgloss.Color("#4a4556")
	formAccent = lipgloss.Color("#ff97d7")
	formError  = lipgloss.Color("#d94f74")
)

type formStyles struct {
	base  lipgloss.Style
	title lipgloss.Style
	muted lipgloss.Style
	dim   lipgloss.Style
	err   lipgloss.Style
	panel lipgloss.Style
}

func newFormStyles() formStyles {
	base := lipgloss.NewStyle().Foreground(formFg)
	return formStyles{
		base:  base,
		title: base.Foreground(formAccent).Bold(true),
		muted: base.Foreground(formMuted),
		dim:   base.Foreground(formDim),
		err:   base.Foreground(formError).Bold(true),
		panel: base.Border(lipgloss.HiddenBorder()).Padding(1, 2),
	}
}

// inputBody renders a single-field form page (title text, project name).
func (s formStyles) inputBody(heading, subtitle, label string, input textinput.Model, errMsg, footer string) string {
	field := s.base.
		Padding(0, 1).
		Width(max(input.Width()+2, 24)).
		Render(input.View())
	parts := []string{
		s.title.Render(heading),
		s.muted.Render(subtitle),
		"",
		s.dim.Render(label),
		field,
	}
	if errMsg != "" {
		parts = append(parts, "", s.err.Render(errMsg))
	}
	parts = append(parts, "", s.dim.Render(footer))
	return s.panel.Render(strings.Join(parts, "\n"))
}

// pagerBody renders the FIGlet style pager with the MOTD divider treatment.
func (s formStyles) pagerBody(heading, subtitle string, font Font, selected, total int, previewSrc, errMsg string, width, height int, footer string) string {
	previewWidth := width - 14
	if previewWidth > 100 {
		previewWidth = 100
	}
	if previewWidth < 24 {
		previewWidth = 24
	}
	previewHeight := height - 12
	if previewHeight < 4 {
		previewHeight = 4
	}
	preview := fitPreview(previewSrc, previewWidth, max(previewHeight-2, 1))
	dividerPeak := lipgloss.Darken(formAccent, 0.22)
	if preview == "" && errMsg == "" {
		preview = s.dim.Render("rendering preview…")
	} else {
		preview = s.base.Foreground(dividerPeak).Bold(true).Render(preview)
	}
	// Transparent context: the gradient still fades toward formBg, but no
	// cell background is painted, so the divider row blends into whatever
	// background the terminal shows around the panel.
	divider := ui.GlowRule{
		Context: ui.NewContext(ui.Palette{}, formBg, true),
		Width:   previewWidth,
		Glyph:   "━",
		Peak:    dividerPeak,
	}.Render()
	previewBlock := lipgloss.NewStyle().Width(lipgloss.Width(preview)).Align(lipgloss.Left).Render(preview)
	preview = lipgloss.JoinVertical(lipgloss.Center, previewBlock, "", divider)
	counter := fmt.Sprintf("%d / %d", selected+1, total)
	header := lipgloss.JoinHorizontal(
		lipgloss.Top,
		s.title.Render(font.Name),
		s.base.Width(max(previewWidth-lipgloss.Width(font.Name)-lipgloss.Width(counter), 1)).Render(""),
		s.muted.Render(counter),
	)
	parts := []string{
		s.title.Render(heading),
		s.muted.Render(subtitle),
		"",
		header,
		s.base.Width(previewWidth).Height(previewHeight).Align(lipgloss.Center, lipgloss.Center).Render(preview),
	}
	if errMsg != "" {
		parts = append(parts, s.err.Render(errMsg))
	}
	parts = append(parts, s.dim.Render(footer))
	return s.panel.Render(strings.Join(parts, "\n"))
}

// canvas paints the body onto a terminal-sized Lip Gloss canvas so the whole
// window carries the form background (see the Bubble Tea rendering rules).
func (s formStyles) canvas(body string, width, height int) tea.View {
	painted := lipgloss.Place(
		max(width, lipgloss.Width(body)),
		max(height, lipgloss.Height(body)),
		lipgloss.Center,
		lipgloss.Center,
		body,
		lipgloss.WithWhitespaceStyle(s.base),
	)
	view := tea.NewView(painted)
	view.AltScreen = true
	view.BackgroundColor = formBg
	return view
}

func fitPreview(preview string, width, height int) string {
	lines := strings.Split(preview, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}
