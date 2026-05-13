package confirm

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestModelConfirmYes(t *testing.T) {
	t.Parallel()

	m := NewModel([]string{"a.txt"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	state := updated.(model)
	if !state.confirmed {
		t.Fatal("expected confirmation to be true")
	}
}

func TestModelConfirmNo(t *testing.T) {
	t.Parallel()

	m := NewModel([]string{"a.txt"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	state := updated.(model)
	if state.confirmed {
		t.Fatal("expected confirmation to be false")
	}
}

func TestModelToggleSelection(t *testing.T) {
	t.Parallel()

	m := NewModel([]string{"a.txt"})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	state := updated.(model)
	if state.cursorYes {
		t.Fatal("expected cursor to move to no")
	}
}
