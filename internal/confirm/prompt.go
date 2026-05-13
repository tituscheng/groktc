package confirm

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type PromptResult struct {
	Confirmed bool
}

type model struct {
	files     []string
	cursorYes bool
	done      bool
	confirmed bool
}

func NewModel(files []string) tea.Model {
	return model{files: append([]string(nil), files...), cursorYes: true}
}

func Prompt(input io.Reader, output io.Writer, files []string) (bool, error) {
	program := tea.NewProgram(NewModel(files), tea.WithInput(input), tea.WithOutput(output))
	finalModel, err := program.Run()
	if err != nil {
		return false, fmt.Errorf("run confirmation prompt: %w", err)
	}

	result, ok := finalModel.(model)
	if !ok {
		return false, fmt.Errorf("unexpected confirmation model type %T", finalModel)
	}

	return result.confirmed, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.done = true
			m.confirmed = false
			return m, tea.Quit
		case "left", "h":
			m.cursorYes = true
		case "right", "l":
			m.cursorYes = false
		case "tab":
			m.cursorYes = !m.cursorYes
		case "enter", " ":
			m.done = true
			m.confirmed = m.cursorYes
			return m, tea.Quit
		case "y", "Y":
			m.done = true
			m.confirmed = true
			return m, tea.Quit
		case "n", "N":
			m.done = true
			m.confirmed = false
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString("The following text files were found in the current directory:\n\n")
	for _, file := range m.files {
		b.WriteString("  - ")
		b.WriteString(file)
		b.WriteByte('\n')
	}

	b.WriteString("\nAnalyze these files for xAI token estimates?\n\n")
	if m.cursorYes {
		b.WriteString("  [Yes]   No\n")
	} else {
		b.WriteString("   Yes   [No]\n")
	}
	b.WriteString("\nUse left/right, tab, y/n, or enter.\n")

	return b.String()
}
