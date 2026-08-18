package wizard

import (
	"fmt"
	"prelude/internal/motd"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// motdStyle is the active MOTD portion of a setup result. Background and
// sizing values are already formatted as Nix expressions so both generated
// config templates can share the same data.
const (
	motdPreviewCanvasWidth = 80 // reference width for helper tests
	motdPreviewSampleText  = "acme"
	// Keep the MOTD style preview independent from the selected title font. The
	// bundled mini font has a compact, stable geometry that fits the static
	// preview canvas while the selected font remains responsible for title.txt.
	motdPreviewFontName = "mini"
)

type motdStyle struct {
	Align            string
	VerticalAlign    string
	TitleAlign       string
	Border           bool
	Background       string
	ClearScreen      bool
	MarginX          int
	MarginY          int
	MarginMinHeight  int
	PaddingX         int
	PaddingY         int
	PaddingMinHeight int
	Width            string
	MaxWidth         string
}

type motdSpacingPreset struct {
	Name      string
	Hint      string
	X         int
	Y         int
	MinHeight int
}

type motdWidthPreset struct {
	Name         string
	Hint         string
	WidthExpr    string // Nix expression for the template
	MaxWidthExpr string // Nix expression for the template
	PreviewWidth int    // legacy static-canvas width (used by helper tests)
	WidthInt     int    // motd.Config Width: 0 = full terminal
	MaxWidthInt  int    // motd.Config MaxWidth: 0 = unbounded
}

type motdBackgroundOption struct {
	Mode  string
	Label string
	Hint  string
	Token string
}

var motdAlignments = []string{"left", "center", "right"}
var motdVerticalAlignments = []string{"top", "center", "bottom"}
var motdTitleAlignments = []string{"left", "center", "right"}

var motdMarginPresets = []motdSpacingPreset{
	{Name: "compact", Hint: "2 columns, no vertical gap", X: 2, Y: 0},
	{Name: "balanced", Hint: "4 columns, 2 rows; tall screens breathe", X: 4, Y: 2, MinHeight: 40},
	{Name: "spacious", Hint: "6 columns, 3 rows", X: 6, Y: 3},
}

var motdPaddingPresets = []motdSpacingPreset{
	{Name: "compact", Hint: "tight content inset", X: 1, Y: 0},
	{Name: "roomy", Hint: "4 columns, 2 rows", X: 4, Y: 2},
	{Name: "generous", Hint: "6 columns, 3 rows", X: 6, Y: 3},
}

var motdWidthPresets = []motdWidthPreset{
	{Name: "compact", Hint: "fixed 80-column card", WidthExpr: "80", MaxWidthExpr: "80", PreviewWidth: 48, WidthInt: 80, MaxWidthInt: 80},
	{Name: "wide", Hint: "full terminal, capped at 100", WidthExpr: `"full"`, MaxWidthExpr: "100", PreviewWidth: 64, WidthInt: 0, MaxWidthInt: 100},
	{Name: "full", Hint: "full terminal, no cap", WidthExpr: `"full"`, MaxWidthExpr: "null", PreviewWidth: 72, WidthInt: 0, MaxWidthInt: 0},
}

var motdBackgroundOptions = []motdBackgroundOption{
	{Mode: "transparent", Label: "transparent", Hint: "let the terminal show through"},
	{Mode: "theme", Label: "theme background", Hint: "use the selected theme bg token", Token: "bg"},
	{Mode: "surface", Label: "surface color", Hint: "use the selected theme surface", Token: "surface"},
	{Mode: "accent", Label: "accent color", Hint: "use the selected theme accent", Token: "accent"},
}

const (
	motdLayoutAlignField = iota
	motdLayoutVerticalField
	motdLayoutTitleField
)

const (
	motdSpacingMarginField = iota
	motdSpacingPaddingField
	motdSpacingWidthField
)

const (
	motdSurfaceCardField = iota
	motdSurfaceBorderField
	motdSurfaceClearField
)

func defaultMotdStyle() motdStyle {
	return motdStyle{
		Align:           "center",
		VerticalAlign:   "bottom",
		TitleAlign:      "center",
		Border:          false,
		Background:      "true",
		ClearScreen:     true,
		MarginX:         4,
		MarginY:         2,
		MarginMinHeight: 40,
		PaddingX:        4,
		PaddingY:        2,
		Width:           `"full"`,
		MaxWidth:        "100",
	}
}

func (m wizardModel) selectedTheme() Theme {
	if len(m.cfg.Themes) == 0 {
		return Theme{}
	}
	index := m.themeIndex
	if index < 0 || index >= len(m.cfg.Themes) {
		index = 0
	}
	return m.cfg.Themes[index]
}

func (m wizardModel) selectedMotdStyle() motdStyle {
	margin := motdMarginPresets[boundedIndex(m.motdMarginIndex, len(motdMarginPresets))]
	padding := motdPaddingPresets[boundedIndex(m.motdPaddingIndex, len(motdPaddingPresets))]
	width := motdWidthPresets[boundedIndex(m.motdWidthIndex, len(motdWidthPresets))]
	return motdStyle{
		Align:            motdAlignments[boundedIndex(m.motdAlignIndex, len(motdAlignments))],
		VerticalAlign:    motdVerticalAlignments[boundedIndex(m.motdVerticalAlignIndex, len(motdVerticalAlignments))],
		TitleAlign:       motdTitleAlignments[boundedIndex(m.motdTitleAlignIndex, len(motdTitleAlignments))],
		Border:           m.motdBorder,
		Background:       m.motdBackgroundNix(m.motdBackgroundIndex),
		ClearScreen:      m.motdClearScreen,
		MarginX:          margin.X,
		MarginY:          margin.Y,
		MarginMinHeight:  margin.MinHeight,
		PaddingX:         padding.X,
		PaddingY:         padding.Y,
		PaddingMinHeight: padding.MinHeight,
		Width:            width.WidthExpr,
		MaxWidth:         width.MaxWidthExpr,
	}
}

func (m wizardModel) motdBackgroundNix(index int) string {
	option := motdBackgroundOptions[boundedIndex(index, len(motdBackgroundOptions))]
	switch option.Mode {
	case "transparent":
		return "false"
	case "theme":
		return "true"
	default:
		if color, ok := m.selectedTheme().Palette[option.Token]; ok && color != "" {
			return nixString(color)
		}
		// Sparse test/config palettes still get a valid, theme-derived fill.
		return "true"
	}
}

func (m wizardModel) motdBackgroundLabel(index int) string {
	option := motdBackgroundOptions[boundedIndex(index, len(motdBackgroundOptions))]
	if option.Token == "" {
		return option.Label
	}
	if color := m.selectedTheme().Palette[option.Token]; color != "" {
		return fmt.Sprintf("%s %s", option.Label, color)
	}
	return option.Label
}

func boundedIndex(index, length int) int {
	if length <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func cycleIndex(index, delta, length int) int {
	if length <= 0 {
		return 0
	}
	return wrap(index+delta, length)
}

func (m wizardModel) firstStepAfterComponents() wizardStep {
	if m.components[componentMotd] {
		return stepMotdContent
	}
	return stepTheme
}

func (m wizardModel) updateMotdSurface(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.motdSurfaceCursor = cycleIndex(m.motdSurfaceCursor, -1, 3)
	case "down", "j", "tab":
		m.motdSurfaceCursor = cycleIndex(m.motdSurfaceCursor, 1, 3)
	case "left", "h":
		m.cycleMotdSurfaceValue(-1)
	case "right", "l", "space":
		m.cycleMotdSurfaceValue(1)
	case "enter":
		m.step = stepMotdSpacing
	case "esc", "backspace":
		m.step = stepMotdContent
		m.focusMotdContentField(m.lastMotdContentPhase())
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *wizardModel) cycleMotdLayoutValue(delta int) {
	switch m.motdLayoutCursor {
	case motdLayoutAlignField:
		m.motdAlignIndex = cycleIndex(m.motdAlignIndex, delta, len(motdAlignments))
	case motdLayoutVerticalField:
		m.motdVerticalAlignIndex = cycleIndex(m.motdVerticalAlignIndex, delta, len(motdVerticalAlignments))
	case motdLayoutTitleField:
		m.motdTitleAlignIndex = cycleIndex(m.motdTitleAlignIndex, delta, len(motdTitleAlignments))
	}
}

func (m wizardModel) updateMotdSpacing(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.motdSpacingCursor = cycleIndex(m.motdSpacingCursor, -1, 3)
	case "down", "j", "tab":
		m.motdSpacingCursor = cycleIndex(m.motdSpacingCursor, 1, 3)
	case "left", "h":
		m.cycleMotdSpacingValue(-1)
	case "right", "l", "space":
		m.cycleMotdSpacingValue(1)
	case "enter":
		m.step = stepMotdLayout
	case "esc", "backspace":
		m.step = stepMotdSurface
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *wizardModel) cycleMotdSpacingValue(delta int) {
	switch m.motdSpacingCursor {
	case motdSpacingMarginField:
		m.motdMarginIndex = cycleIndex(m.motdMarginIndex, delta, len(motdMarginPresets))
	case motdSpacingPaddingField:
		m.motdPaddingIndex = cycleIndex(m.motdPaddingIndex, delta, len(motdPaddingPresets))
	case motdSpacingWidthField:
		m.motdWidthIndex = cycleIndex(m.motdWidthIndex, delta, len(motdWidthPresets))
	}
}

func (m wizardModel) updateMotdLayout(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.motdLayoutCursor = cycleIndex(m.motdLayoutCursor, -1, 3)
	case "down", "j", "tab":
		m.motdLayoutCursor = cycleIndex(m.motdLayoutCursor, 1, 3)
	case "left", "h":
		m.cycleMotdLayoutValue(-1)
	case "right", "l", "space":
		m.cycleMotdLayoutValue(1)
	case "enter":
		m.step = stepTheme
	case "esc", "backspace":
		m.step = stepMotdSpacing
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *wizardModel) cycleMotdSurfaceValue(delta int) {
	switch m.motdSurfaceCursor {
	case motdSurfaceCardField:
		m.motdBackgroundIndex = cycleIndex(m.motdBackgroundIndex, delta, len(motdBackgroundOptions))
	case motdSurfaceBorderField:
		m.motdBorder = !m.motdBorder
	case motdSurfaceClearField:
		m.motdClearScreen = !m.motdClearScreen
	}
}

func (m wizardModel) motdLayoutRows(s formStyles) []string {
	return []string{
		motdSettingRow(s, m.motdLayoutCursor == motdLayoutAlignField, "block", motdAlignments[m.motdAlignIndex], "horizontal placement"),
		motdSettingRow(s, m.motdLayoutCursor == motdLayoutVerticalField, "vertical", motdVerticalAlignments[m.motdVerticalAlignIndex], "placement after clear"),
		motdSettingRow(s, m.motdLayoutCursor == motdLayoutTitleField, "title", motdTitleAlignments[m.motdTitleAlignIndex], "wordmark alignment"),
		"",
		m.motdPreview(),
	}
}

func (m wizardModel) motdSpacingRows(s formStyles) []string {
	margin := motdMarginPresets[m.motdMarginIndex]
	padding := motdPaddingPresets[m.motdPaddingIndex]
	width := motdWidthPresets[m.motdWidthIndex]
	return []string{
		motdSettingRow(s, m.motdSpacingCursor == motdSpacingMarginField, "margin", margin.Name, margin.Hint),
		motdSettingRow(s, m.motdSpacingCursor == motdSpacingPaddingField, "padding", padding.Name, padding.Hint),
		motdSettingRow(s, m.motdSpacingCursor == motdSpacingWidthField, "width", width.Name, width.Hint),
		"",
		m.motdPreview(),
	}
}

func (m wizardModel) motdSurfaceRows(s formStyles) []string {
	return []string{
		motdSettingRow(s, m.motdSurfaceCursor == motdSurfaceCardField, "card bg", m.motdBackgroundLabel(m.motdBackgroundIndex), motdBackgroundOptions[m.motdBackgroundIndex].Hint),
		motdSettingRow(s, m.motdSurfaceCursor == motdSurfaceBorderField, "border", onOff(m.motdBorder), "rounded frame around the card"),
		motdSettingRow(s, m.motdSurfaceCursor == motdSurfaceClearField, "clear screen", onOff(m.motdClearScreen), "erase before rendering"),
		"",
		m.motdPreview(),
	}
}

func motdSettingRow(s formStyles, selected bool, label, value, hint string) string {
	return listRow(s, selected, fmt.Sprintf("%-13s", label)+s.title.Render(value)+"  "+s.muted.Render(hint))
}

func onOff(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func (m wizardModel) motdPreview() string {
	if len(m.cfg.Themes) == 0 {
		return newFormStyles().dim.Render("MOTD preview unavailable until a theme is loaded")
	}

	margin := motdMarginPresets[boundedIndex(m.motdMarginIndex, len(motdMarginPresets))]
	padding := motdPaddingPresets[boundedIndex(m.motdPaddingIndex, len(motdPaddingPresets))]

	// Size the preview to the terminal, deducting the panel chrome: padding
	// (2 rows, 4 cols), heading + subtitle + blank + blank + footer (5 rows),
	// and the 3 setting rows + separating blank (4 rows) that sit above the
	// preview in every motd style step.
	previewWidth := max(m.width-4, 40)
	previewHeight := max(m.height-11, 8)

	// Render through the real MOTDView so the preview matches shell-entry output.
	cfg := m.motdPreviewConfig()
	rendered := motd.Render(motd.RenderInput{
		Config:         cfg,
		Cache:          motd.Cache{}, // cold cache: statuses show "pending"
		TerminalWidth:  previewWidth,
		TerminalHeight: previewHeight,
	})
	rendered = fitPreview(rendered, previewWidth, previewHeight)

	s := newFormStyles()
	meta := fmt.Sprintf(
		"%s block · %s vertical · border %s · %s margin · %s padding · %s card · clear %s",
		motdAlignments[boundedIndex(m.motdAlignIndex, len(motdAlignments))],
		motdVerticalAlignments[boundedIndex(m.motdVerticalAlignIndex, len(motdVerticalAlignments))],
		onOff(m.motdBorder),
		margin.Name,
		padding.Name,
		motdBackgroundOptions[boundedIndex(m.motdBackgroundIndex, len(motdBackgroundOptions))].Label,
		onOff(m.motdClearScreen),
	)
	meta = fitPreview(meta, previewWidth, 1)
	header := fitPreview("live MOTD preview", previewWidth, 1)
	return strings.Join([]string{s.muted.Render(header), rendered, s.dim.Render(meta)}, "\n")
}

func alignPreviewTitleBlock(title string, width int, align string) string {
	lines := strings.Split(title, "\n")
	blockWidth := min(lipgloss.Width(title), max(width, 1))
	available := max(width-blockWidth, 0)
	offset := 0
	switch align {
	case "center":
		offset = available / 2
	case "right":
		offset = available
	}
	for index, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < blockWidth {
			line += strings.Repeat(" ", blockWidth-lineWidth)
		}
		lines[index] = lipgloss.NewStyle().
			Width(width).
			Align(lipgloss.Left).
			Render(strings.Repeat(" ", offset) + line)
	}
	return strings.Join(lines, "\n")
}

func previewCardDimensions(previewWidth, desiredWidth, margin, padding int, border bool) (cardWidth, contentWidth, paddingX int) {
	previewWidth = max(previewWidth, 3)
	margin = max(margin, 0)
	padding = max(padding, 0)
	gutter := min(margin, max((previewWidth-3)/2, 0))
	cardWidth = min(max(desiredWidth, 3), previewWidth-2*gutter)
	if cardWidth < 3 {
		cardWidth = min(previewWidth, 3)
	}
	paddingX = min(padding, max((cardWidth-3)/2, 0))
	borderWidth := 0
	if border {
		borderWidth = 2
	}
	contentWidth = max(cardWidth-paddingX*2-borderWidth, 1)
	return cardWidth, contentWidth, paddingX
}
