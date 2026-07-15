// PROTOTYPE — delete after deciding whether tea.ExecProcess provides the shell
// handoff behavior wanted by menu-tui.
//
// Question: can Bubble Tea leave its alternate screen, give an interactive
// child shell the existing TTY, then restore and repaint the same TUI when the
// user exits that shell—without manually releasing/restoring the terminal?
//
// Run: cd src && go run ./shell-exec-prototype
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type shellFinishedMsg struct{ err error }

// markedShell decorates an ordinary exec.Cmd without taking ownership of the
// terminal lifecycle. Bubble Tea supplies its stdin/stdout/stderr, then this
// wrapper adds an entry banner and terminal title around the child process.
type markedShell struct{ cmd *exec.Cmd }

func (c *markedShell) SetStdin(r io.Reader)  { c.cmd.Stdin = r }
func (c *markedShell) SetStdout(w io.Writer) { c.cmd.Stdout = w }
func (c *markedShell) SetStderr(w io.Writer) { c.cmd.Stderr = w }

func (c *markedShell) Run() error {
	fmt.Fprint(c.cmd.Stdout,
		"\x1b]0;PROTOTYPE CHILD SHELL — exit returns to TUI\x07",
		"\x1b[2J\x1b[H",
		"\x1b[1;30;45m  PROTOTYPE CHILD SHELL  \x1b[0m\n",
		"\x1b[35mType \x1b[1mexit\x1b[22m to return to the Bubble Tea screen.\x1b[0m\n\n",
	)
	err := c.cmd.Run()
	fmt.Fprint(c.cmd.Stdout, "\x1b]0;shell-exec-prototype\x07")
	return err
}

func childShell(path string) tea.ExecCommand {
	name := filepath.Base(path)
	var args []string
	switch name {
	case "zsh":
		args = []string{"-f"}
	case "bash":
		args = []string{"--noprofile", "--norc", "-i"}
	case "fish":
		args = []string{"--no-config"}
	default:
		args = []string{"-i"}
	}
	cmd := exec.Command(path, args...)
	cmd.Env = append(os.Environ(),
		"PS1=〔PROTOTYPE CHILD — exit returns to TUI〕 ",
		"PROMPT=〔PROTOTYPE CHILD — exit returns to TUI〕 ",
	)
	return &markedShell{cmd: cmd}
}

type model struct {
	shell    string
	launches int
	state    string
	lastErr  string
	width    int
	height   int
}

func initialModel() model {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	return model{
		shell:  shell,
		state:  "ready",
		width:  80,
		height: 24,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case shellFinishedMsg:
		m.state = "returned from shell"
		if msg.err != nil {
			m.lastErr = msg.err.Error()
		} else {
			m.lastErr = "none"
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			m.launches++
			m.state = "shell active (exit to return)"
			m.lastErr = ""

			// ExecProcess handles the complete lifecycle: leave alt-screen,
			// restore cooked mode, connect the child to the TTY, wait, then
			// restore raw mode + alt-screen and repaint this model.
			return m, tea.Exec(childShell(m.shell), func(err error) tea.Msg {
				return shellFinishedMsg{err: err}
			})
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	accent := lipgloss.Color("#ff97d7")
	fg := lipgloss.Color("#d6d2df")
	muted := lipgloss.Color("#8787af")
	dim := lipgloss.Color("#4a4556")
	bg := lipgloss.Color("#0e0d11")

	title := lipgloss.NewStyle().Foreground(accent).Bold(true)
	label := lipgloss.NewStyle().Foreground(muted).Width(12)
	value := lipgloss.NewStyle().Foreground(fg)
	quiet := lipgloss.NewStyle().Foreground(dim)
	key := lipgloss.NewStyle().Foreground(bg).Background(accent).Bold(true)

	row := func(name, val string) string {
		return label.Render(name) + value.Render(val)
	}

	lastErr := m.lastErr
	if lastErr == "" {
		lastErr = "—"
	}
	body := strings.Join([]string{
		title.Render("tea.ExecProcess shell handoff"),
		quiet.Render("PROTOTYPE — press enter, use the shell, then exit"),
		"",
		row("state", m.state),
		row("shell", m.shell),
		row("launches", fmt.Sprint(m.launches)),
		row("last error", lastErr),
		"",
		key.Render(" enter ") + quiet.Render(" open isolated interactive shell") +
			"   " + key.Render(" q ") + quiet.Render(" quit prototype"),
		"",
		quiet.Render("While in the child shell, try resize, clear, colors, or another TUI."),
	}, "\n")

	panel := lipgloss.NewStyle().
		Background(bg).
		Padding(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(dim).
		Render(body)

	content := lipgloss.Place(
		max(m.width, lipgloss.Width(panel)),
		max(m.height, lipgloss.Height(panel)),
		lipgloss.Center,
		lipgloss.Center,
		panel,
	)
	view := tea.NewView(content)
	view.AltScreen = true
	view.WindowTitle = "shell-exec-prototype"
	return view
}

func main() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "shell-exec-prototype:", err)
		os.Exit(1)
	}
}
