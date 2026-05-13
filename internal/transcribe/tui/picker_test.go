package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestFilePickerToggleSpace(t *testing.T) {
	m := filePickerModel{
		files:    []string{"a.mp3", "b.mp4", "c.txt"},
		selected: make(map[int]bool),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	state := updated.(filePickerModel)
	require.True(t, state.selected[0])

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	state = updated.(filePickerModel)
	require.False(t, state.selected[0])
}

func TestFilePickerSelectAll(t *testing.T) {
	m := filePickerModel{
		files:    []string{"a", "b", "c"},
		selected: make(map[int]bool),
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	state := updated.(filePickerModel)
	require.Len(t, state.selected, 3)

	// Pressing 'a' again deselects all.
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	state = updated.(filePickerModel)
	require.Empty(t, state.selected)
}

func TestFilePickerCursorMovement(t *testing.T) {
	m := filePickerModel{
		files:    []string{"a", "b", "c"},
		selected: make(map[int]bool),
	}
	require.Equal(t, 0, m.cursor)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	state := updated.(filePickerModel)
	require.Equal(t, 1, state.cursor)

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyDown})
	state = updated.(filePickerModel)
	require.Equal(t, 2, state.cursor)

	// Down at last item is a no-op.
	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyDown})
	state = updated.(filePickerModel)
	require.Equal(t, 2, state.cursor)

	updated, _ = state.Update(tea.KeyMsg{Type: tea.KeyUp})
	state = updated.(filePickerModel)
	require.Equal(t, 1, state.cursor)
}

func TestFilePickerEnterRequiresSelection(t *testing.T) {
	m := filePickerModel{
		files:    []string{"a", "b"},
		selected: make(map[int]bool),
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(filePickerModel)
	require.False(t, state.confirm, "should not confirm with empty selection")
	require.Nil(t, cmd, "should not quit with empty selection")
	require.NotEmpty(t, state.errMsg)
}

func TestFilePickerEnterConfirmsWithSelection(t *testing.T) {
	m := filePickerModel{
		files:    []string{"a", "b"},
		selected: map[int]bool{0: true},
	}
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	state := updated.(filePickerModel)
	require.True(t, state.confirm)
	require.NotNil(t, cmd, "should issue tea.Quit cmd")
}

func TestFilePickerCancelKeys(t *testing.T) {
	for _, key := range []string{"q", "esc", "ctrl+c"} {
		m := filePickerModel{
			files:    []string{"a"},
			selected: make(map[int]bool),
		}
		var msg tea.KeyMsg
		switch key {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "ctrl+c":
			msg = tea.KeyMsg{Type: tea.KeyCtrlC}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		updated, cmd := m.Update(msg)
		state := updated.(filePickerModel)
		require.True(t, state.canceled, "key %q should cancel", key)
		require.NotNil(t, cmd)
	}
}
