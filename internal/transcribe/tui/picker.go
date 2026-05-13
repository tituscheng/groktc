package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type filePickerModel struct {
	files    []string
	selected map[int]bool
	cursor   int
	canceled bool
	confirm  bool
	errMsg   string
}

// PickFiles shows a TUI file picker with multi-select. Returns the selected
// paths or ErrCanceled if the user cancels.
func PickFiles(input io.Reader, output io.Writer, available []string) ([]string, error) {
	if len(available) == 0 {
		return nil, fmt.Errorf("no files available")
	}
	m := filePickerModel{
		files:    append([]string(nil), available...),
		selected: make(map[int]bool),
	}
	program := tea.NewProgram(m, tea.WithInput(input), tea.WithOutput(output))
	finalModel, err := program.Run()
	if err != nil {
		return nil, fmt.Errorf("run file picker: %w", err)
	}
	state, ok := finalModel.(filePickerModel)
	if !ok {
		return nil, fmt.Errorf("unexpected picker state %T", finalModel)
	}
	if state.canceled || !state.confirm {
		return nil, ErrCanceled
	}
	out := make([]string, 0, len(state.selected))
	for i, f := range state.files {
		if state.selected[i] {
			out = append(out, f)
		}
	}
	return out, nil
}

func (m filePickerModel) Init() tea.Cmd { return nil }

func (m filePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.errMsg = ""
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.canceled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case " ", "x":
			m.selected[m.cursor] = !m.selected[m.cursor]
			if !m.selected[m.cursor] {
				delete(m.selected, m.cursor)
			}
		case "a":
			allSelected := len(m.selected) == len(m.files)
			if allSelected {
				m.selected = make(map[int]bool)
			} else {
				for i := range m.files {
					m.selected[i] = true
				}
			}
		case "enter":
			if len(m.selected) == 0 {
				m.errMsg = "select at least one file (space to toggle)"
				return m, nil
			}
			m.confirm = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m filePickerModel) View() string {
	var b strings.Builder
	b.WriteString(tuiHeader("Select files to transcribe"))
	b.WriteString("\n\n")

	for i, f := range m.files {
		cursor := "  "
		if i == m.cursor {
			cursor = tuiCursor("> ")
		}
		check := "[ ]"
		if m.selected[i] {
			check = tuiCheck("[x]")
		}
		fmt.Fprintf(&b, "%s%s %s\n", cursor, check, filepath.Base(f))
	}

	b.WriteString("\n")
	if m.errMsg != "" {
		b.WriteString(tuiCheck("! "))
		b.WriteString(m.errMsg)
		b.WriteString("\n")
	}
	b.WriteString(tuiMuted("[↑/↓] move  [space] toggle  [a] all/none  [enter] confirm  [q] cancel"))
	b.WriteString("\n")
	return b.String()
}
