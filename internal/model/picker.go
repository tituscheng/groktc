package model

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type pickerModel struct {
	models        []CatalogModel
	selectedModel string
	cursor        int
	done          bool
	canceled      bool
	chosenModelID string
}

func newPickerModel(models []CatalogModel, selectedModel string) pickerModel {
	cursor := 0
	if selectedModel != "" {
		for idx, item := range models {
			if item.ID == selectedModel {
				cursor = idx
				break
			}
		}
	}
	return pickerModel{
		models:        append([]CatalogModel(nil), models...),
		selectedModel: selectedModel,
		cursor:        cursor,
	}
}

func promptModelSelection(input io.Reader, output io.Writer, models []CatalogModel, selectedModel string) (string, bool, error) {
	program := tea.NewProgram(newPickerModel(models, selectedModel), tea.WithInput(input), tea.WithOutput(output))
	finalModel, err := program.Run()
	if err != nil {
		return "", false, fmt.Errorf("run model selection prompt: %w", err)
	}
	state, ok := finalModel.(pickerModel)
	if !ok {
		return "", false, fmt.Errorf("unexpected model selection state %T", finalModel)
	}
	if state.canceled {
		return "", true, nil
	}
	return state.chosenModelID, false, nil
}

func (m pickerModel) Init() tea.Cmd {
	return nil
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.canceled = true
			m.done = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.models)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.models) == 0 {
				m.canceled = true
			} else {
				m.chosenModelID = m.models[m.cursor].ID
			}
			m.done = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m pickerModel) View() string {
	var b strings.Builder
	b.WriteString(colorHeader("Select default xAI model for groktc"))
	b.WriteString("\n\n")
	if len(m.models) == 0 {
		b.WriteString("No models available.\n")
		return b.String()
	}

	for idx, item := range m.models {
		pointer := " "
		if idx == m.cursor {
			pointer = colorCurrent(">")
		}

		currentMarker := ""
		if item.ID == m.selectedModel {
			currentMarker = " " + colorCurrent("[current]")
		}

		retirement := item.RetirementLabel()
		if retirement != "" {
			retirement = " " + colorWarn("["+retirement+"]")
		}

		price := formatPriceSummary(item)
		line := fmt.Sprintf("%s %s%s%s\n", pointer, colorModel(item.ID), currentMarker, retirement)
		b.WriteString(line)
		if price != "" {
			b.WriteString("    ")
			b.WriteString(price)
			b.WriteByte('\n')
		}
		if item.BestFor != "" {
			b.WriteString("    ")
			b.WriteString(colorHeader("best for:"))
			b.WriteString(" ")
			b.WriteString(item.BestFor)
			b.WriteByte('\n')
		}
	}

	b.WriteString("\n")
	b.WriteString(colorMuted("Use up/down and Enter. Press q to cancel."))
	b.WriteString("\n")
	return b.String()
}
