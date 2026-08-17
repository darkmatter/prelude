package portal

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"prelude/pkg/shared"
	"prelude/pkg/ui"
)

// statusesMsg carries one completed probe sweep back into the model.
type statusesMsg map[string]Status

// tickMsg drives the periodic re-probe.
type tickMsg struct{}

type tuiModel struct {
	cfg       Config
	context   ui.Context
	selection *Selection
	statuses  map[string]Status
	rows      []Row
	cursor    int
	width     int
	height    int
	probing   bool
	lastSweep time.Time
}

func newTUIModel(cfg Config) tuiModel {
	background := lipgloss.Color(cfg.Palette.Bg.String())
	return tuiModel{
		cfg:       cfg,
		context:   ui.NewContext(cfg.Palette, background, false),
		selection: NewSelection(cfg.Apps),
		statuses:  map[string]Status{},
		// Seeded rather than left at zero: the first paint happens before any
		// WindowSizeMsg arrives, and placing the panel into a 0x0 canvas
		// renders an empty screen until the user happens to resize.
		width:  80,
		height: 24,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(probeCmd(m.cfg), tickCmd())
}

// probeCmd runs one sweep off the update loop. Probes are network-bound and
// would otherwise block rendering for the whole timeout budget.
func probeCmd(cfg Config) tea.Cmd {
	return func() tea.Msg {
		prober := NewProber(cfg.Timeout())
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout()+time.Second)
		defer cancel()
		return statusesMsg(prober.ProbeAll(ctx, cfg.Apps))
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case statusesMsg:
		m.statuses = map[string]Status(msg)
		m.probing = false
		m.lastSweep = time.Now()
		m.rows = Rows(m.cfg.Apps, m.selection, m.statuses)
		return m, nil

	case tickMsg:
		return m, tea.Batch(probeCmd(m.cfg), tickCmd())

	case tea.KeyPressMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m tuiModel) onKey(key tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	rows := m.visibleRows()

	switch key.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "down", "j":
		if m.cursor < len(rows)-1 {
			m.cursor++
		}
		return m, nil

	// Environment selection is per row, not global: local chat against a
	// deployed platform is a normal thing to want.
	case "right", "l", "tab":
		if row, ok := m.rowAt(rows, m.cursor); ok {
			m.selection.Cycle(row.App, 1)
			m.rows = Rows(m.cfg.Apps, m.selection, m.statuses)
		}
		return m, nil

	case "left", "h", "shift+tab":
		if row, ok := m.rowAt(rows, m.cursor); ok {
			m.selection.Cycle(row.App, -1)
			m.rows = Rows(m.cfg.Apps, m.selection, m.statuses)
		}
		return m, nil

	case "enter", "o":
		if row, ok := m.rowAt(rows, m.cursor); ok && row.Environment.URL != "" {
			return m, openURL(row.Environment.URL)
		}
		return m, nil

	case "r":
		m.probing = true
		return m, probeCmd(m.cfg)
	}
	return m, nil
}

func (m tuiModel) visibleRows() []Row {
	if m.rows != nil {
		return m.rows
	}
	return Rows(m.cfg.Apps, m.selection, m.statuses)
}

func (m tuiModel) rowAt(rows []Row, index int) (Row, bool) {
	if index < 0 || index >= len(rows) {
		return Row{}, false
	}
	return rows[index], true
}

// openURL hands the URL to the platform opener. Errors are swallowed on
// purpose: a missing opener must not take down the portal, and the row still
// renders a clickable OSC 8 link as the fallback path.
func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		opener := "xdg-open"
		if runtime.GOOS == "darwin" {
			opener = "open"
		}
		_ = exec.Command(opener, url).Start()
		return nil
	}
}

func (m tuiModel) statusStyle(state State) lipgloss.Style {
	switch state {
	case StateUp:
		return m.context.Success()
	case StateDown:
		return m.context.Error()
	case StateGated:
		return m.context.Warning()
	default:
		return m.context.Dim()
	}
}

func (m tuiModel) View() tea.View {
	rows := m.visibleRows()
	width := m.contentWidth()

	var lines []string
	lines = append(lines,
		m.context.Accent().Bold(true).Render(strings.ToUpper(m.cfg.Project)+" PORTAL"),
		m.context.Muted().Render("launch an app · ←/→ switches environment"),
		"",
	)

	if len(rows) == 0 {
		lines = append(lines,
			m.context.Dim().Render("no apps configured — set prelude.portal.apps"),
		)
	}

	for index, row := range rows {
		// Each element must be exactly one visual line: PanelFrame.Paint frames
		// a single line, so a multi-line row would escape the border.
		lines = append(lines, m.renderRow(row, index == m.cursor)...)
	}

	frame := ui.PanelFrame{Context: m.context}.WithSize(width)
	filler := m.context.Fill()

	painted := []string{frame.Top(), frame.Blank()}
	for _, line := range lines {
		painted = append(painted, frame.Paint(line, filler))
	}
	painted = append(painted, frame.Blank(), frame.Bottom(), m.footer(width))
	panel := strings.Join(painted, "\n")

	canvas := lipgloss.Place(
		max(m.width, 1), max(m.height, 1),
		lipgloss.Center, lipgloss.Center,
		panel,
		lipgloss.WithWhitespaceStyle(m.context.Fill()),
	)

	view := tea.NewView(canvas)
	view.BackgroundColor = m.context.Background
	return view
}

func (m tuiModel) contentWidth() int {
	width := m.cfg.MaxWidth
	if width <= 0 {
		width = 76
	}
	if m.width > 0 && m.width-4 < width {
		width = m.width - 4
	}
	return max(width, 30)
}

func (m tuiModel) renderRow(row Row, selected bool) []string {
	light := m.statusStyle(row.Status.State).Render(row.Status.Light())

	name := row.App.Name
	nameStyle := m.context.Foreground()
	if selected {
		nameStyle = m.context.Accent().Bold(true)
	}

	env := row.Environment.Name
	if row.EnvironmentCount > 1 {
		env = fmt.Sprintf("%s (%d/%d)", env, row.EnvironmentIndex+1, row.EnvironmentCount)
	}

	link := ui.Link{
		Context: m.context,
		Label:   row.Environment.URL,
		URL:     row.Environment.URL,
	}.Render()

	cursor := "  "
	if selected {
		cursor = m.context.Accent().Render("▸ ")
	}

	head := fmt.Sprintf("%s%s %s  %s",
		cursor, light,
		nameStyle.Render(pad(name, 12)),
		m.context.Muted().Render(pad(env, 16)),
	)
	detail := m.context.Dim().Render(detailText(row.Status))

	// Link on its own line: URLs are long, and truncating the thing you are
	// meant to click defeats the purpose.
	return []string{head + detail, "    " + link}
}

func (m tuiModel) footer(width int) string {
	hints := []ui.KeyHint{
		{Key: "↵", Text: "open"},
		{Key: "←→", Text: "env"},
		{Key: "r", Text: "refresh"},
		{Key: "q", Text: "quit"},
	}

	stamp := "probing…"
	if !m.probing && !m.lastSweep.IsZero() {
		stamp = "checked " + m.lastSweep.Format("15:04:05")
	}

	return ui.KeyHintsFooter{
		Context:           m.context,
		Width:             width,
		Outer:             m.context.Background,
		Open:              m.context.Background,
		HorizontalPadding: 2,
	}.Render(hints, stamp)
}

func pad(value string, width int) string {
	if len(value) >= width {
		return value
	}
	return value + strings.Repeat(" ", width-len(value))
}

// RunTUI starts the terminal front end.
func RunTUI(cfg Config) error {
	options := []tea.ProgramOption{}
	if profile, ok := shared.ConfiguredColorProfile(cfg.ColorProfile); ok {
		options = append(options, tea.WithColorProfile(profile))
	}
	_, err := tea.NewProgram(newTUIModel(cfg), options...).Run()
	return err
}
