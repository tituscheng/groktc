package markdown

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tituscheng/groktc/pkg/runner"
	"github.com/tituscheng/groktc/internal/style"

	"github.com/stretchr/testify/require"
)

type memoryPromptCache struct {
	name    string
	content string
	mode    CacheMode
	prompts []SavedPrompt
}

func (m *memoryPromptCache) SavePrompt(_ context.Context, name string, content string) error {
	m.name = name
	m.content = content
	m.mode = CacheModeCreate
	return nil
}

func (m *memoryPromptCache) ListPrompts(_ context.Context) ([]SavedPrompt, error) {
	return append([]SavedPrompt(nil), m.prompts...), nil
}

func (m *memoryPromptCache) GetPromptByName(_ context.Context, name string) (SavedPrompt, bool, error) {
	for _, prompt := range m.prompts {
		if prompt.Name == name {
			return prompt, true, nil
		}
	}
	return SavedPrompt{}, false, nil
}

func (m *memoryPromptCache) UpsertPrompt(_ context.Context, name string, content string) error {
	m.name = name
	m.content = content
	m.mode = CacheModeUpdate
	return nil
}

func (m *memoryPromptCache) DeletePrompt(_ context.Context, name string) error {
	for i, p := range m.prompts {
		if p.Name == name {
			m.prompts = append(m.prompts[:i], m.prompts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("prompt %q not found", name)
}

func (m *memoryPromptCache) Close() error { return nil }

type fakeProcessor struct {
	mu            sync.Mutex
	failFor       map[string]error
	active        int
	maxActive     int
	processed     []string
	sleepDuration time.Duration
}

func (f *fakeProcessor) ProcessFile(_ context.Context, task FileTask, _ string) error {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()

	if f.sleepDuration > 0 {
		time.Sleep(f.sleepDuration)
	}

	f.mu.Lock()
	f.active--
	f.processed = append(f.processed, task.InputPath)
	err := f.failFor[task.InputPath]
	f.mu.Unlock()
	return err
}

func TestRunnerPromptPrecedence(t *testing.T) {
	dir := t.TempDir()
	promptFile := filepath.Join(dir, "prompt.txt")
	require.NoError(t, os.WriteFile(promptFile, []byte("from file"), 0o644))

	runner := NewRunner()
	runner.PromptProvider = func(context.Context) (GUIPromptResult, error) {
		return GUIPromptResult{Text: "from gui"}, nil
	}

	prompt, _, err := ResolvePrompt(context.Background(), "from flag", promptFile, false, runner.PromptProvider)
	require.NoError(t, err)
	require.Equal(t, "from flag", prompt)

	prompt, _, err = ResolvePrompt(context.Background(), "", promptFile, false, runner.PromptProvider)
	require.NoError(t, err)
	require.Equal(t, "from file", prompt)
}

func TestRunnerNoGUIRequiresPrompt(t *testing.T) {
	runner := NewRunner()
	_, _, err := ResolvePrompt(context.Background(), "", "", true, runner.PromptProvider)
	require.Error(t, err)
}

func TestRunnerGUIPromptCanBeCached(t *testing.T) {
	cache := &memoryPromptCache{}
	runner := NewRunner()
	runner.CacheFactory = func() (PromptCache, error) { return cache, nil }
	runner.PromptProvider = func(context.Context) (GUIPromptResult, error) {
		return GUIPromptResult{Text: "Prompt", Cache: true, CacheName: "Reusable", CacheMode: CacheModeCreate}, nil
	}

	prompt, result, err := ResolvePrompt(context.Background(), "", "", false, runner.PromptProvider)
	require.NoError(t, err)
	require.Equal(t, "Prompt", prompt)
	require.True(t, result.Cache)

	require.NoError(t, PersistPrompt(context.Background(), cache, result))
	require.Equal(t, "Reusable", cache.name)
	require.Equal(t, "Prompt", cache.content)
	require.Equal(t, CacheModeCreate, cache.mode)
}

func TestRunnerGUIPromptUpdateUsesUpsert(t *testing.T) {
	cache := &memoryPromptCache{}
	runner := NewRunner()
	runner.CacheFactory = func() (PromptCache, error) { return cache, nil }

	result := GUIPromptResult{
		Text:      "Prompt",
		Cache:     true,
		CacheName: "Reusable",
		CacheMode: CacheModeUpdate,
	}

	require.NoError(t, PersistPrompt(context.Background(), cache, result))
	require.Equal(t, "Reusable", cache.name)
	require.Equal(t, "Prompt", cache.content)
	require.Equal(t, CacheModeUpdate, cache.mode)
}

func TestRunnerLoadSavedPrompts(t *testing.T) {
	cache := &memoryPromptCache{
		prompts: []SavedPrompt{
			{Name: "One", Content: "first"},
			{Name: "Two", Content: "second"},
		},
	}
	runner := NewRunner()
	runner.CacheFactory = func() (PromptCache, error) { return cache, nil }

	prompts, err := ListSavedPrompts(context.Background(), cache)
	require.NoError(t, err)
	require.Len(t, prompts, 2)
	require.Equal(t, "One", prompts[0].Name)
}

func TestRunnerProcessTasksContinuesAfterFailure(t *testing.T) {
	processor := &fakeProcessor{
		failFor: map[string]error{"bad.txt": fmt.Errorf("failed")},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	r := NewRunner()
	r.Stdout = &stdout
	r.Stderr = &stderr

	summary := runner.ProcessTasks(context.Background(), []FileTask{
		{InputPath: "ok.txt", OutputPath: "ok.md"},
		{InputPath: "bad.txt", OutputPath: "bad.md"},
	}, func(t FileTask) runner.Task { return t }, func(ctx context.Context, task FileTask) error {
		return processor.ProcessFile(ctx, task, "Prompt")
	}, runner.Options{
		Concurrency: DefaultConcurrency,
		Stdout:      r.Stdout,
		Stderr:      r.Stderr,
		Style:       runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		PanicLabel:  "markdown",
		TaskLabel:   "markdown",
	})

	require.Len(t, summary.Processed, 1)
	require.Len(t, summary.Failed, 1)
	require.Contains(t, stderr.String(), "Failed bad.txt")
}

func TestRunnerProcessTasksLimitsConcurrency(t *testing.T) {
	processor := &fakeProcessor{sleepDuration: 25 * time.Millisecond}
	r := NewRunner()
	r.Stdout = io.Discard
	r.Stderr = io.Discard

	tasks := make([]FileTask, 12)
	for index := range tasks {
		tasks[index] = FileTask{InputPath: fmt.Sprintf("%d.txt", index), OutputPath: fmt.Sprintf("%d.md", index)}
	}

	summary := runner.ProcessTasks(context.Background(), tasks, func(t FileTask) runner.Task { return t }, func(ctx context.Context, task FileTask) error {
		return processor.ProcessFile(ctx, task, "Prompt")
	}, runner.Options{
		Concurrency: DefaultConcurrency,
		Stdout:      r.Stdout,
		Stderr:      r.Stderr,
		Style:       runner.Style{Header: style.Header, Label: style.Label, OK: style.OK, Skip: style.Skip, Fail: style.Fail, Muted: style.Muted, Value: style.Value, Total: style.Total},
		PanicLabel:  "markdown",
		TaskLabel:   "markdown",
	})
	require.Len(t, summary.Processed, 12)
	require.LessOrEqual(t, processor.maxActive, DefaultConcurrency)
}
