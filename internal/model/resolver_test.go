package model

import (
	"context"
	"testing"
)

type fakeStore struct {
	selected string
	models   map[string]CatalogModel
}

func (f *fakeStore) SetSelectedModel(_ context.Context, modelID string) error {
	f.selected = modelID
	return nil
}

func (f *fakeStore) GetSelectedModel(_ context.Context) (string, error) {
	return f.selected, nil
}

func (f *fakeStore) UpsertModelCache(_ context.Context, models []CatalogModel) error {
	if f.models == nil {
		f.models = make(map[string]CatalogModel)
	}
	for _, item := range models {
		f.models[item.ID] = item
	}
	return nil
}

func (f *fakeStore) ListModelCache(_ context.Context) ([]CatalogModel, error) {
	out := make([]CatalogModel, 0, len(f.models))
	for _, item := range f.models {
		out = append(out, item)
	}
	return out, nil
}

func (f *fakeStore) GetModelByID(_ context.Context, modelID string) (CatalogModel, bool, error) {
	item, ok := f.models[modelID]
	return item, ok, nil
}

func (f *fakeStore) Close() error { return nil }

func TestResolverExplicitModelOverride(t *testing.T) {
	t.Parallel()

	resolver := Resolver{
		Store:        &fakeStore{},
		DefaultModel: DefaultModelID,
	}

	result, err := resolver.ResolveForTokenize(context.Background(), "grok-4.3", true)
	if err != nil {
		t.Fatalf("ResolveForTokenize returned error: %v", err)
	}
	if result.ModelID != "grok-4.3" {
		t.Fatalf("expected explicit model, got %q", result.ModelID)
	}
}

func TestResolverExplicitRetiredModelFails(t *testing.T) {
	t.Parallel()

	resolver := Resolver{
		Store:        &fakeStore{},
		DefaultModel: DefaultModelID,
	}

	_, err := resolver.ResolveForTokenize(context.Background(), "grok-4-fast-reasoning", true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResolverUsesSelectedModelWhenNotExplicit(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		selected: "grok-4.3",
		models: map[string]CatalogModel{
			"grok-4.3": {ID: "grok-4.3"},
		},
	}

	resolver := Resolver{
		Store:        store,
		DefaultModel: DefaultModelID,
	}

	result, err := resolver.ResolveForTokenize(context.Background(), "", false)
	if err != nil {
		t.Fatalf("ResolveForTokenize returned error: %v", err)
	}
	if result.ModelID != "grok-4.3" {
		t.Fatalf("expected selected model, got %q", result.ModelID)
	}
}

func TestResolverRetiredSelectedModelFallsBack(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		selected: "grok-4-fast-reasoning",
		models: map[string]CatalogModel{
			"grok-4-fast-reasoning": {ID: "grok-4-fast-reasoning"},
		},
	}

	resolver := Resolver{
		Store:        store,
		DefaultModel: DefaultModelID,
	}

	result, err := resolver.ResolveForTokenize(context.Background(), "", false)
	if err != nil {
		t.Fatalf("ResolveForTokenize returned error: %v", err)
	}
	if result.ModelID != DefaultModelID {
		t.Fatalf("expected fallback model %q, got %q", DefaultModelID, result.ModelID)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected fallback warning")
	}
}
