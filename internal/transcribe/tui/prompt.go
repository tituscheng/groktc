package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// PromptInteractive walks the user through choosing or creating a prompt.
//
// Flow:
//  1. List saved prompts + "<New prompt>" → user selects.
//  2a. If existing: ask "Use as-is" / "Edit" / "Cancel".
//  2b. If new: open $EDITOR (defaults to vi) for the user to type.
//  3. If new or edited, ask "Use Once" / "Save & Use" / "Cancel".
//  4. If saving, prompt for a name.
//
// Returns ErrCanceled if the user cancels at any step.
func PromptInteractive(input io.Reader, output io.Writer, saved []SavedPromptOption) (PromptResult, error) {
	choice, err := showPromptList(input, output, saved)
	if err != nil {
		return PromptResult{}, err
	}

	if choice.isExisting {
		action, err := askExistingAction(input, output, choice.name)
		if err != nil {
			return PromptResult{}, err
		}
		switch action {
		case existingActionUse:
			return PromptResult{Text: choice.content, CacheMode: CacheModeNone}, nil
		case existingActionEdit:
			return editAndSave(input, output, choice.content, choice.name)
		default:
			return PromptResult{}, ErrCanceled
		}
	}

	// New prompt: open editor for blank input.
	return editAndSave(input, output, "", "")
}

// editAndSave opens $EDITOR with initialContent, then asks how to use the result.
// existingName, if non-empty, is the name of a saved prompt that the result
// will update.
func editAndSave(input io.Reader, output io.Writer, initialContent, existingName string) (PromptResult, error) {
	text, err := editInEditor(initialContent)
	if err != nil {
		return PromptResult{}, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return PromptResult{}, fmt.Errorf("prompt is empty — nothing to do")
	}

	if existingName != "" {
		// Editing an existing saved prompt → ask whether to update or use once.
		action, err := askUpdateAction(input, output, existingName)
		if err != nil {
			return PromptResult{}, err
		}
		switch action {
		case updateActionUseOnce:
			return PromptResult{Text: text, CacheMode: CacheModeNone}, nil
		case updateActionUpdate:
			return PromptResult{
				Text:      text,
				Cache:     true,
				CacheName: existingName,
				CacheMode: CacheModeUpdate,
			}, nil
		default:
			return PromptResult{}, ErrCanceled
		}
	}

	// New prompt → ask whether to save it.
	action, err := askSaveAction(input, output)
	if err != nil {
		return PromptResult{}, err
	}
	switch action {
	case saveActionUseOnce:
		return PromptResult{Text: text, CacheMode: CacheModeNone}, nil
	case saveActionSave:
		name, err := readLine(input, output, "Prompt name: ")
		if err != nil {
			return PromptResult{}, err
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return PromptResult{}, fmt.Errorf("prompt name is required")
		}
		return PromptResult{
			Text:      text,
			Cache:     true,
			CacheName: name,
			CacheMode: CacheModeCreate,
		}, nil
	default:
		return PromptResult{}, ErrCanceled
	}
}

// editInEditor writes initialContent to a temp file, opens it in $EDITOR
// (default: vi), and returns the file's contents after the editor exits.
func editInEditor(initialContent string) (string, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}

	f, err := os.CreateTemp("", "groktc-prompt-*.md")
	if err != nil {
		return "", fmt.Errorf("create temp prompt file: %w", err)
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)

	if initialContent != "" {
		if _, err := f.WriteString(initialContent); err != nil {
			f.Close()
			return "", fmt.Errorf("write initial prompt: %w", err)
		}
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	cmd := exec.Command(editor, tmpPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor %q failed: %w", editor, err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read edited prompt: %w", err)
	}
	return string(data), nil
}

func readLine(input io.Reader, output io.Writer, prompt string) (string, error) {
	fmt.Fprint(output, prompt)
	r := bufio.NewReader(input)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// ----- prompt-list bubbletea model -----

type promptChoice struct {
	name       string
	content    string
	isExisting bool
}

type promptListModel struct {
	options  []promptChoice
	cursor   int
	canceled bool
	chose    bool
}

func showPromptList(input io.Reader, output io.Writer, saved []SavedPromptOption) (promptChoice, error) {
	options := make([]promptChoice, 0, len(saved)+1)
	options = append(options, promptChoice{name: "<New prompt>", isExisting: false})
	for _, p := range saved {
		options = append(options, promptChoice{
			name:       p.Name,
			content:    p.Content,
			isExisting: true,
		})
	}

	program := tea.NewProgram(
		promptListModel{options: options},
		tea.WithInput(input), tea.WithOutput(output),
	)
	finalModel, err := program.Run()
	if err != nil {
		return promptChoice{}, fmt.Errorf("run prompt list: %w", err)
	}
	state, ok := finalModel.(promptListModel)
	if !ok {
		return promptChoice{}, fmt.Errorf("unexpected list state %T", finalModel)
	}
	if state.canceled || !state.chose {
		return promptChoice{}, ErrCanceled
	}
	return state.options[state.cursor], nil
}

func (m promptListModel) Init() tea.Cmd { return nil }

func (m promptListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.canceled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.chose = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m promptListModel) View() string {
	var b strings.Builder
	b.WriteString(tuiHeader("Choose a prompt"))
	b.WriteString("\n\n")
	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = tuiCursor("> ")
		}
		fmt.Fprintf(&b, "%s%s", cursor, opt.name)
		if opt.isExisting {
			preview := promptPreview(opt.content)
			if preview != "" {
				fmt.Fprintf(&b, " %s", tuiPreview("("+preview+")"))
			}
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(tuiMuted("[↑/↓] move  [enter] select  [q] cancel"))
	b.WriteString("\n")
	return b.String()
}

func promptPreview(content string) string {
	trimmed := strings.Join(strings.Fields(strings.TrimSpace(content)), " ")
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 60 {
		return trimmed
	}
	return trimmed[:57] + "..."
}

// ----- existing-prompt action bubbletea model -----

type existingAction int

const (
	existingActionCancel existingAction = iota
	existingActionUse
	existingActionEdit
)

type existingActionModel struct {
	name     string
	cursor   int
	canceled bool
	chose    bool
}

func askExistingAction(input io.Reader, output io.Writer, name string) (existingAction, error) {
	program := tea.NewProgram(
		existingActionModel{name: name, cursor: 0},
		tea.WithInput(input), tea.WithOutput(output),
	)
	finalModel, err := program.Run()
	if err != nil {
		return existingActionCancel, fmt.Errorf("run existing action: %w", err)
	}
	state := finalModel.(existingActionModel)
	if state.canceled || !state.chose {
		return existingActionCancel, ErrCanceled
	}
	switch state.cursor {
	case 0:
		return existingActionUse, nil
	case 1:
		return existingActionEdit, nil
	default:
		return existingActionCancel, nil
	}
}

func (m existingActionModel) Init() tea.Cmd { return nil }

func (m existingActionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.canceled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < 2 {
				m.cursor++
			}
		case "enter":
			m.chose = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m existingActionModel) View() string {
	options := []string{
		"Use as-is",
		"Edit before using",
		"Cancel",
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n\n", tuiHeader("Selected:"), m.name)
	for i, opt := range options {
		cursor := "  "
		if i == m.cursor {
			cursor = tuiCursor("> ")
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, opt)
	}
	b.WriteString("\n")
	b.WriteString(tuiMuted("[↑/↓] move  [enter] select  [q] cancel"))
	b.WriteString("\n")
	return b.String()
}

// ----- update action (after editing existing) -----

type updateAction int

const (
	updateActionCancel updateAction = iota
	updateActionUseOnce
	updateActionUpdate
)

func askUpdateAction(input io.Reader, output io.Writer, name string) (updateAction, error) {
	options := []string{
		fmt.Sprintf("Update saved prompt %q and use", name),
		"Use Once (don't update saved)",
		"Cancel",
	}
	idx, err := simpleSelect(input, output, "Edit complete. What would you like to do?", options)
	if err != nil {
		return updateActionCancel, err
	}
	switch idx {
	case 0:
		return updateActionUpdate, nil
	case 1:
		return updateActionUseOnce, nil
	default:
		return updateActionCancel, nil
	}
}

// ----- save action (for new prompts) -----

type saveAction int

const (
	saveActionCancel saveAction = iota
	saveActionUseOnce
	saveActionSave
)

func askSaveAction(input io.Reader, output io.Writer) (saveAction, error) {
	options := []string{
		"Save to library and use",
		"Use Once (don't save)",
		"Cancel",
	}
	idx, err := simpleSelect(input, output, "Prompt entered. What would you like to do?", options)
	if err != nil {
		return saveActionCancel, err
	}
	switch idx {
	case 0:
		return saveActionSave, nil
	case 1:
		return saveActionUseOnce, nil
	default:
		return saveActionCancel, nil
	}
}

// simpleSelect runs a generic single-choice TUI selector.
func simpleSelect(input io.Reader, output io.Writer, header string, options []string) (int, error) {
	program := tea.NewProgram(
		selectModel{header: header, options: options},
		tea.WithInput(input), tea.WithOutput(output),
	)
	finalModel, err := program.Run()
	if err != nil {
		return -1, fmt.Errorf("run selector: %w", err)
	}
	state := finalModel.(selectModel)
	if state.canceled || !state.chose {
		return -1, ErrCanceled
	}
	return state.cursor, nil
}

type selectModel struct {
	header   string
	options  []string
	cursor   int
	canceled bool
	chose    bool
}

func (m selectModel) Init() tea.Cmd { return nil }

func (m selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.canceled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter":
			m.chose = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m selectModel) View() string {
	var b strings.Builder
	b.WriteString(tuiHeader(m.header))
	b.WriteString("\n\n")
	for i, opt := range m.options {
		cursor := "  "
		if i == m.cursor {
			cursor = tuiCursor("> ")
		}
		fmt.Fprintf(&b, "%s%s\n", cursor, opt)
	}
	b.WriteString("\n")
	b.WriteString(tuiMuted("[↑/↓] move  [enter] select  [q] cancel"))
	b.WriteString("\n")
	return b.String()
}

