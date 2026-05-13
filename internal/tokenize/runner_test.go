package tokenize

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	modelcfg "github.com/tituscheng/groktc/internal/model"
	"github.com/tituscheng/groktc/pkg/xai"
)

type fakeClient struct {
	counts    map[string]int
	lastModel string
}

func (f *fakeClient) CountTokens(_ context.Context, model string, text string) (int, error) {
	f.lastModel = model
	return f.counts[text], nil
}

type fakeStore struct {
	selected string
	models   map[string]modelcfg.CatalogModel
}

func (f *fakeStore) SetSelectedModel(_ context.Context, modelID string) error {
	f.selected = modelID
	return nil
}

func (f *fakeStore) GetSelectedModel(_ context.Context) (string, error) {
	return f.selected, nil
}

func (f *fakeStore) UpsertModelCache(_ context.Context, models []modelcfg.CatalogModel) error {
	if f.models == nil {
		f.models = make(map[string]modelcfg.CatalogModel)
	}
	for _, item := range models {
		f.models[item.ID] = item
	}
	return nil
}

func (f *fakeStore) ListModelCache(_ context.Context) ([]modelcfg.CatalogModel, error) {
	out := make([]modelcfg.CatalogModel, 0, len(f.models))
	for _, item := range f.models {
		out = append(out, item)
	}
	return out, nil
}

func (f *fakeStore) GetModelByID(_ context.Context, modelID string) (modelcfg.CatalogModel, bool, error) {
	model, ok := f.models[modelID]
	return model, ok, nil
}

func (f *fakeStore) Close() error { return nil }

func TestRunnerRunExplicitFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "a.txt")
	second := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(first, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	if err := os.WriteFile(second, []byte("world"), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}

	var stdout bytes.Buffer
	client := &fakeClient{counts: map[string]int{"hello": 5, "world": 6}}
	runner := &Runner{
		Client: client,
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(_ io.Reader, _ io.Writer, files []string) (bool, error) {
			return true, nil
		},
		StoreFactory: func() (modelcfg.Store, error) {
			return &fakeStore{}, nil
		},
	}

	err := withWorkingDir(dir, func() error {
		return runner.Run(context.Background(), Options{
			Args:          []string{"b.txt", "a.txt"},
			Model:         "grok-4.3",
			ModelExplicit: true,
		})
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.lastModel != "grok-4.3" {
		t.Fatalf("expected explicit model grok-4.3, got %q", client.lastModel)
	}

	output := stdout.String()
	if !strings.Contains(output, "a.txt") || !strings.Contains(output, "b.txt") {
		t.Fatalf("expected file names in output, got %q", output)
	}
	if !strings.Contains(output, "Total input tokens") || !strings.Contains(output, "11") {
		t.Fatalf("expected total tokens in output, got %q", output)
	}
	if !strings.Contains(output, "Estimated output tokens (best guess)") {
		t.Fatalf("expected output token estimate in output, got %q", output)
	}
	if !strings.Contains(output, "Estimated total cost (input + output)") {
		t.Fatalf("expected total cost estimate in output, got %q", output)
	}
}

func TestRunnerResolveFilesNonTTY(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var stdout bytes.Buffer
	runner := &Runner{
		Client: &fakeClient{},
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(_ io.Reader, _ io.Writer, files []string) (bool, error) {
			return true, nil
		},
		StoreFactory: func() (modelcfg.Store, error) {
			return &fakeStore{}, nil
		},
	}

	err := withWorkingDir(dir, func() error {
		_, err := runner.resolveFiles(Options{Model: "grok-4.3"})
		return err
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "requires a TTY") {
		t.Fatalf("expected TTY error, got %v", err)
	}
	if !strings.Contains(stdout.String(), "a.txt") {
		t.Fatalf("expected discovered file list, got %q", stdout.String())
	}
}

func TestRunnerResolveFilesWithYes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	runner := &Runner{
		Client: &fakeClient{},
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(_ io.Reader, _ io.Writer, files []string) (bool, error) {
			return true, nil
		},
		StoreFactory: func() (modelcfg.Store, error) {
			return &fakeStore{}, nil
		},
	}

	var files []string
	err := withWorkingDir(dir, func() error {
		var err error
		files, err = runner.resolveFiles(Options{Model: "grok-4.3", Yes: true})
		return err
	})
	if err != nil {
		t.Fatalf("resolveFiles returned error: %v", err)
	}
	if len(files) != 1 || !strings.HasSuffix(files[0], "a.txt") {
		t.Fatalf("unexpected files: %v", files)
	}
}

func TestFormatUSD(t *testing.T) {
	t.Parallel()

	got := formatUSD(xai.EstimateInputCostUSD(2000, xai.Pricing{InputPerMillionUSD: 1.25}))
	if got != "$0.002500" {
		t.Fatalf("unexpected formatted cost: %s", got)
	}
}

func TestEstimateOutputTokens(t *testing.T) {
	t.Parallel()

	if got := estimateOutputTokens(0); got != 0 {
		t.Fatalf("expected 0 for empty input, got %d", got)
	}
	if got := estimateOutputTokens(10); got != 64 {
		t.Fatalf("expected minimum 64 output tokens, got %d", got)
	}
	if got := estimateOutputTokens(1000); got != 350 {
		t.Fatalf("expected scaled estimate 350, got %d", got)
	}
}

func TestRunnerUsesSelectedModelWhenFlagNotExplicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	client := &fakeClient{counts: map[string]int{"hello": 5}}
	store := &fakeStore{
		selected: "grok-4.20-reasoning",
		models: map[string]modelcfg.CatalogModel{
			"grok-4.20-reasoning": {
				ID:                      "grok-4.20-reasoning",
				PromptTextTokenPriceRaw: "10000",
			},
		},
	}

	runner := &Runner{
		Client: client,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(_ io.Reader, _ io.Writer, files []string) (bool, error) {
			return true, nil
		},
		StoreFactory: func() (modelcfg.Store, error) {
			return store, nil
		},
	}

	err := withWorkingDir(dir, func() error {
		return runner.Run(context.Background(), Options{
			Args:  []string{"a.txt"},
			Model: "grok-4.3",
		})
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.lastModel != "grok-4.20-reasoning" {
		t.Fatalf("expected selected model to be used, got %q", client.lastModel)
	}
}

func TestRunnerExplicitModelOverridesSelectedModel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	client := &fakeClient{counts: map[string]int{"hello": 5}}
	store := &fakeStore{
		selected: "grok-4.20-reasoning",
		models: map[string]modelcfg.CatalogModel{
			"grok-4.20-reasoning": {
				ID:                      "grok-4.20-reasoning",
				PromptTextTokenPriceRaw: "10000",
			},
		},
	}

	runner := &Runner{
		Client: client,
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		Stdin:  bytes.NewBuffer(nil),
		LookupEnv: func(string) string {
			return "test-key"
		},
		TerminalChecker: func() bool { return false },
		Prompt: func(_ io.Reader, _ io.Writer, files []string) (bool, error) {
			return true, nil
		},
		StoreFactory: func() (modelcfg.Store, error) {
			return store, nil
		},
	}

	err := withWorkingDir(dir, func() error {
		return runner.Run(context.Background(), Options{
			Args:          []string{"a.txt"},
			Model:         "grok-4.3",
			ModelExplicit: true,
		})
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if client.lastModel != "grok-4.3" {
		t.Fatalf("expected explicit model to be used, got %q", client.lastModel)
	}
}

func withWorkingDir(dir string, fn func() error) error {
	previous, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer os.Chdir(previous)
	return fn()
}
