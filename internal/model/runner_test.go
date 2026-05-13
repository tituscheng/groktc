package model

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/tituscheng/groktc/pkg/xai"
)

type fakeCatalogClient struct {
	models []xai.LanguageModel
	err    error
}

func (f fakeCatalogClient) ListLanguageModels(_ context.Context) ([]xai.LanguageModel, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]xai.LanguageModel(nil), f.models...), nil
}

type memoryStore struct {
	selected string
	models   map[string]CatalogModel
}

func (m *memoryStore) SetSelectedModel(_ context.Context, modelID string) error {
	m.selected = modelID
	return nil
}

func (m *memoryStore) GetSelectedModel(_ context.Context) (string, error) {
	return m.selected, nil
}

func (m *memoryStore) UpsertModelCache(_ context.Context, models []CatalogModel) error {
	if m.models == nil {
		m.models = make(map[string]CatalogModel)
	}
	for _, item := range models {
		m.models[item.ID] = item
	}
	return nil
}

func (m *memoryStore) ListModelCache(_ context.Context) ([]CatalogModel, error) {
	out := make([]CatalogModel, 0, len(m.models))
	for _, item := range m.models {
		out = append(out, item)
	}
	SortModels(out)
	return out, nil
}

func (m *memoryStore) GetModelByID(_ context.Context, modelID string) (CatalogModel, bool, error) {
	item, ok := m.models[modelID]
	return item, ok, nil
}

func (m *memoryStore) Close() error { return nil }

func TestRunnerNonTTYListMode(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		selected: "grok-4.3",
		models: map[string]CatalogModel{
			"grok-4.3": {
				ID:                          "grok-4.3",
				PromptTextTokenPriceRaw:     "12500",
				CompletionTextTokenPriceRaw: "25000",
				BestFor:                     "best for general tasks",
			},
		},
	}

	var stdout bytes.Buffer
	runner := &Runner{
		Client: fakeCatalogClient{},
		StoreFactory: func() (Store, error) {
			return store, nil
		},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return ""
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(io.Reader, io.Writer, []CatalogModel, string) (string, bool, error) {
			return "", false, nil
		},
	}

	if err := runner.Run(context.Background(), RunnerOptions{}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "Current model: grok-4.3") {
		t.Fatalf("expected current model in output, got %q", out)
	}
}

func TestRunnerSetModelRejectsRetired(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		models: map[string]CatalogModel{
			"grok-4-fast-reasoning": {
				ID:               "grok-4-fast-reasoning",
				RetirementStatus: RetirementStatusRetired,
				ReplacementModel: "grok-4.3",
			},
		},
	}

	runner := &Runner{
		Client: fakeCatalogClient{
			models: []xai.LanguageModel{
				{ID: "grok-4-fast-reasoning"},
			},
		},
		StoreFactory: func() (Store, error) {
			return store, nil
		},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(io.Reader, io.Writer, []CatalogModel, string) (string, bool, error) {
			return "", false, nil
		},
	}

	err := runner.Run(context.Background(), RunnerOptions{SetModelID: "grok-4-fast-reasoning"})
	if err == nil {
		t.Fatal("expected retired-model error")
	}
}

func TestRunnerUsesCacheWhenAPIUnavailable(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		models: map[string]CatalogModel{
			"grok-4.3": {ID: "grok-4.3", BestFor: "best for reasoning"},
		},
	}

	var stdout bytes.Buffer
	runner := &Runner{
		Client: fakeCatalogClient{err: errors.New("network failed")},
		StoreFactory: func() (Store, error) {
			return store, nil
		},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(io.Reader, io.Writer, []CatalogModel, string) (string, bool, error) {
			return "", false, nil
		},
	}

	if err := runner.Run(context.Background(), RunnerOptions{ListOnly: true}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !strings.Contains(stdout.String(), "Model catalog source: cache") {
		t.Fatalf("expected cache source in output, got %q", stdout.String())
	}
}

func TestRunnerFiltersRetiredModelsFromList(t *testing.T) {
	t.Parallel()

	store := &memoryStore{
		models: map[string]CatalogModel{
			"grok-4.3": {
				ID:                          "grok-4.3",
				PromptTextTokenPriceRaw:     "12500",
				CompletionTextTokenPriceRaw: "25000",
			},
			"grok-4-fast-reasoning": {
				ID:               "grok-4-fast-reasoning",
				RetirementStatus: RetirementStatusRetired,
				ReplacementModel: "grok-4.3",
			},
		},
	}

	var stdout bytes.Buffer
	runner := &Runner{
		Client: fakeCatalogClient{err: errors.New("network failed")},
		StoreFactory: func() (Store, error) {
			return store, nil
		},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(io.Reader, io.Writer, []CatalogModel, string) (string, bool, error) {
			return "", false, nil
		},
	}

	if err := runner.Run(context.Background(), RunnerOptions{ListOnly: true}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	out := stdout.String()
	if strings.Contains(out, "grok-4-fast-reasoning") {
		t.Fatalf("expected retired model to be filtered out, output: %q", out)
	}
	if !strings.Contains(out, "grok-4.3") {
		t.Fatalf("expected active model to remain, output: %q", out)
	}
}
