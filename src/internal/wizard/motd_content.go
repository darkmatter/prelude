package wizard

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type motdContentPhase uint8

const (
	motdContentTagline motdContentPhase = iota
	motdContentDescription
	motdContentStatus
	motdContentDevServerURL
)

const (
	motdDefaultTagline                = "Fancy devshells for your nix flake"
	motdDevServerHealthURLPlaceholder = `${APP_HOST:-http://127.0.0.1:3000}/health`
	motdDevServerHealthCommandDefault = `curl -fsS "${APP_HOST:-http://127.0.0.1:3000}/health"`
	motdContentStatusFlakeCursor      = 0
	motdContentStatusDevServerCursor  = 1
	motdContentStatusOptionCount      = 2
)

var motdAppHostHealthURLPattern = regexp.MustCompile(
	`^\$\{APP_HOST:-https?://[A-Za-z0-9._~:/?#\[\]@!&'()*+,;=%-]+\}[A-Za-z0-9._~:/?#\[\]@!&'()*+,;=%-]*$`,
)

type wizardMotdContent struct {
	Tagline            string
	Description        string
	NixFlakeCheck      bool
	DevServerStatus    bool
	DevServerHealthURL string
}

func defaultWizardMotdDescription(project string) string {
	project = strings.TrimSpace(project)
	if project == "" {
		project = "project"
	}
	return fmt.Sprintf(
		"You are now in the %s dev environment, powered by Nix Flakes. All required dependencies, project scripts, and documentation are available in this environment.",
		project,
	)
}

func defaultWizardMotdContent(project string) wizardMotdContent {
	return wizardMotdContent{
		Tagline:            motdDefaultTagline,
		Description:        defaultWizardMotdDescription(project),
		NixFlakeCheck:      true,
		DevServerHealthURL: motdDevServerHealthURLPlaceholder,
	}
}

func (m *wizardModel) beginMotdContent() {
	m.focusMotdContentField(motdContentTagline)
}

func (m *wizardModel) focusMotdContentField(phase motdContentPhase) {
	m.motdContentPhase = phase
	m.motdContentInput.Blur()
	m.motdDescriptionInput.Blur()

	switch phase {
	case motdContentTagline:
		m.motdContentInput.Placeholder = motdDefaultTagline
		m.motdContentInput.SetValue(m.motdContent.Tagline)
		m.motdContentInput.CursorEnd()
		m.motdContentInput.Focus()
	case motdContentDescription:
		m.motdDescriptionInput.Placeholder = defaultWizardMotdDescription(m.projectInput.Value())
		m.motdDescriptionInput.SetValue(m.motdContent.Description)
		m.motdDescriptionInput.MoveToEnd()
		m.motdDescriptionInput.Focus()
	case motdContentStatus:
		// The status page is a pair of toggles rather than a text field.
	case motdContentDevServerURL:
		if strings.TrimSpace(m.motdContent.DevServerHealthURL) == "" {
			m.motdContent.DevServerHealthURL = motdDevServerHealthURLPlaceholder
		}
		m.motdContentInput.Placeholder = motdDevServerHealthURLPlaceholder
		m.motdContentInput.SetValue(m.motdContent.DevServerHealthURL)
		m.motdContentInput.CursorEnd()
		m.motdContentInput.Focus()
	}
	m.err = ""
}

func (m wizardModel) updateMotdContent(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.motdContentPhase {
	case motdContentDescription:
		return m.updateMotdDescription(msg)
	case motdContentStatus:
		return m.updateMotdStatus(msg)
	}

	switch msg.String() {
	case "enter":
		return m.commitMotdContentField()
	case "esc":
		switch m.motdContentPhase {
		case motdContentTagline:
			m.motdContentInput.Blur()
			m.step = stepComponents
		case motdContentDevServerURL:
			m.focusMotdContentField(motdContentStatus)
		}
		return m, nil
	}

	before := m.motdContentInput.Value()
	var cmd tea.Cmd
	m.motdContentInput, cmd = m.motdContentInput.Update(msg)
	if m.motdContentInput.Value() != before {
		m.err = ""
	}
	return m, cmd
}

func (m wizardModel) updateMotdDescription(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		return m.commitMotdContentField()
	case "shift+enter":
		// Insert a newline: forward a plain enter to the textarea so its cursor
		// management stays consistent, without committing the field.
		newline := msg
		newline.Mod = 0
		before := m.motdDescriptionInput.Value()
		var cmd tea.Cmd
		m.motdDescriptionInput, cmd = m.motdDescriptionInput.Update(newline)
		if m.motdDescriptionInput.Value() != before {
			m.err = ""
		}
		return m, cmd
	case "ctrl+d":
		return m.commitMotdContentField()
	case "esc":
		m.focusMotdContentField(motdContentTagline)
		return m, nil
	}

	before := m.motdDescriptionInput.Value()
	var cmd tea.Cmd
	m.motdDescriptionInput, cmd = m.motdDescriptionInput.Update(msg)
	if m.motdDescriptionInput.Value() != before {
		m.err = ""
	}
	return m, cmd
}

func (m wizardModel) updateMotdStatus(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k", "shift+tab":
		m.motdStatusCursor = cycleIndex(m.motdStatusCursor, -1, motdContentStatusOptionCount)
	case "down", "j", "tab":
		m.motdStatusCursor = cycleIndex(m.motdStatusCursor, 1, motdContentStatusOptionCount)
	case "left", "right", "h", "l", "space", "x":
		m.toggleMotdStatus()
	case "enter":
		if m.motdContent.DevServerStatus {
			m.focusMotdContentField(motdContentDevServerURL)
		} else {
			m.step = stepMotdSurface
		}
	case "esc", "backspace":
		m.focusMotdContentField(motdContentDescription)
	case "q":
		m.canceled = true
		return m, tea.Quit
	}
	return m, nil
}

func (m *wizardModel) toggleMotdStatus() {
	switch m.motdStatusCursor {
	case motdContentStatusFlakeCursor:
		m.motdContent.NixFlakeCheck = !m.motdContent.NixFlakeCheck
	case motdContentStatusDevServerCursor:
		m.motdContent.DevServerStatus = !m.motdContent.DevServerStatus
		if m.motdContent.DevServerStatus && strings.TrimSpace(m.motdContent.DevServerHealthURL) == "" {
			m.motdContent.DevServerHealthURL = motdDevServerHealthURLPlaceholder
		}
	}
}

func (m wizardModel) commitMotdContentField() (tea.Model, tea.Cmd) {
	switch m.motdContentPhase {
	case motdContentTagline:
		m.motdContent.Tagline = strings.TrimSpace(m.motdContentInput.Value())
		m.focusMotdContentField(motdContentDescription)
	case motdContentDescription:
		m.motdContent.Description = strings.TrimSpace(m.motdDescriptionInput.Value())
		m.focusMotdContentField(motdContentStatus)
	case motdContentDevServerURL:
		value := strings.TrimSpace(m.motdContentInput.Value())
		if value == "" {
			value = motdDevServerHealthURLPlaceholder
		}
		if err := validateMotdDevServerHealthURL(value); err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.motdContent.DevServerHealthURL = value
		m.motdContentInput.Blur()
		m.step = stepMotdSurface
	}
	return m, nil
}

func (m wizardModel) lastMotdContentPhase() motdContentPhase {
	if m.motdContent.DevServerStatus {
		return motdContentDevServerURL
	}
	return motdContentStatus
}

func (m wizardModel) motdContentBody(s formStyles, step string) string {
	switch m.motdContentPhase {
	case motdContentDescription:
		return s.textareaBody(
			"Write your welcome message (multiline okay)",
			"Shown beneath the tagline in the MOTD.  ·  "+step,
			"WELCOME MESSAGE",
			m.motdDescriptionInput,
			m.err,
			"shift+enter newline  ·  enter next field  ·  esc back",
		)
	case motdContentStatus:
		return s.listBody(
			"Choose live status items",
			"Checks refresh asynchronously without slowing shell startup.  ·  "+step,
			append(m.motdStatusRows(s), "", m.statusLightsPreview(s)),
			m.err,
			"j/k move  ·  space toggle  ·  enter continue  ·  esc back",
		)
	case motdContentDevServerURL:
		return s.inputBody(
			"Set the dev server health URL",
			"Prelude will curl this endpoint asynchronously.  ·  "+step,
			"HEALTH URL",
			m.motdContentInput,
			m.err,
			"enter continue  ·  esc back",
		)
	default:
		return s.inputBody(
			"Write the MOTD tagline",
			"Write a one-line description of your project.  ·  "+step,
			"TAGLINE",
			m.motdContentInput,
			m.err,
			"enter next field  ·  esc back",
		)
	}
}

func (m wizardModel) motdStatusRows(s formStyles) []string {
	rows := []struct {
		enabled bool
		label   string
		hint    string
	}{
		{m.motdContent.NixFlakeCheck, "nix flake check", "recommended; enabled by default"},
		{m.motdContent.DevServerStatus, "dev server", "show health from a URL you provide"},
	}
	result := make([]string, len(rows))
	for index, row := range rows {
		mark := "[ ]"
		if row.enabled {
			mark = "[x]"
		}
		result[index] = listRow(
			s,
			index == m.motdStatusCursor,
			fmt.Sprintf("%-24s", mark+" "+row.label)+s.muted.Render(row.hint),
		)
	}
	return result
}

func (m wizardModel) motdPreviewCommands() string {
	if len(m.commands) == 0 {
		return "no project commands selected"
	}
	count := min(len(m.commands), 3)
	names := make([]string, count)
	for index := range count {
		names[index] = m.commands[index].Name
	}
	return strings.Join(names, "  ·  ")
}

// statusLightsPreview renders the isolated header status lights the user is
// toggling, using the selected theme so the dots and labels match what the
// MOTD will actually paint. Pending checks show the info-colored dot.
func (m wizardModel) statusLightsPreview(s formStyles) string {
	ts := themeSample{theme: m.selectedTheme()}
	var lights []string
	if m.motdContent.NixFlakeCheck {
		lights = append(lights, ts.seg("bg", "info", "● ", false)+ts.seg("bg", "muted", "flake  pending", false))
	}
	if m.motdContent.DevServerStatus {
		lights = append(lights, ts.seg("bg", "info", "● ", false)+ts.seg("bg", "muted", "dev server  pending", false))
	}
	if len(lights) == 0 {
		return s.dim.Render("  (no status lights enabled)")
	}
	return s.dim.Render("  preview  ") + strings.Join(lights, s.dim.Render("  /  "))
}

func validateMotdDevServerHealthURL(value string) error {
	if motdAppHostHealthURLPattern.MatchString(value) {
		return nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("health URL must use http:// or https://, or keep the APP_HOST placeholder")
	}
	return nil
}

func motdDevServerHealthCommand(url string) string {
	url = strings.TrimSpace(url)
	if url == "" {
		url = motdDevServerHealthURLPlaceholder
	}
	if motdAppHostHealthURLPattern.MatchString(url) {
		return `curl -fsS "` + url + `"`
	}
	return "curl -fsS " + shellSingleQuote(url)
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
