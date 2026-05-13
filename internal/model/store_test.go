package model

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tituscheng/groktc/internal/config"
)

func TestOpenStoreCreatesConfigDirAndSchema(t *testing.T) {
	t.Parallel()

	configDir := filepath.Join(t.TempDir(), "nested", "groktc")
	store, err := OpenStore(configDir)
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	defer store.Close()

	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %q to be a directory", configDir)
	}

	dbPath := filepath.Join(configDir, config.DatabaseName)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected db file %q to exist: %v", dbPath, err)
	}
}

func TestStoreSelectedModelRoundTrip(t *testing.T) {
	t.Parallel()

	store := mustOpenStore(t)
	defer store.Close()

	ctx := context.Background()
	if err := store.SetSelectedModel(ctx, "grok-4.3"); err != nil {
		t.Fatalf("SetSelectedModel returned error: %v", err)
	}

	got, err := store.GetSelectedModel(ctx)
	if err != nil {
		t.Fatalf("GetSelectedModel returned error: %v", err)
	}
	if got != "grok-4.3" {
		t.Fatalf("expected selected model grok-4.3, got %q", got)
	}
}

func TestStoreUpsertAndListCache(t *testing.T) {
	t.Parallel()

	store := mustOpenStore(t)
	defer store.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	input := []CatalogModel{
		{
			ID:                          "grok-4.3",
			Aliases:                     []string{"grok-4.3-latest"},
			InputModalities:             []string{"text"},
			OutputModalities:            []string{"text"},
			Version:                     "2026-05-01",
			PromptTextTokenPriceRaw:     "12500",
			CompletionTextTokenPriceRaw: "25000",
			BestFor:                     "best for general tasks",
			LastFetchedAt:               now,
			RawJSON:                     `{"id":"grok-4.3"}`,
		},
	}

	if err := store.UpsertModelCache(ctx, input); err != nil {
		t.Fatalf("UpsertModelCache returned error: %v", err)
	}

	got, err := store.ListModelCache(ctx)
	if err != nil {
		t.Fatalf("ListModelCache returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 cached model, got %d", len(got))
	}
	if got[0].ID != "grok-4.3" {
		t.Fatalf("expected cached model ID grok-4.3, got %q", got[0].ID)
	}
	if got[0].BestFor != "best for general tasks" {
		t.Fatalf("unexpected best_for: %q", got[0].BestFor)
	}

	entry, found, err := store.GetModelByID(ctx, "grok-4.3")
	if err != nil {
		t.Fatalf("GetModelByID returned error: %v", err)
	}
	if !found {
		t.Fatalf("expected model to be found")
	}
	if !strings.Contains(entry.RawJSON, "grok-4.3") {
		t.Fatalf("unexpected raw json: %q", entry.RawJSON)
	}
}

func mustOpenStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := OpenStore(filepath.Join(t.TempDir(), config.DirectoryName))
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	return store
}
