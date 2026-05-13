package model

import "testing"

func TestEnrichModelsAddsBestForAndRetirement(t *testing.T) {
	t.Parallel()

	models := EnrichModels([]CatalogModel{
		{ID: "grok-4.3"},
		{ID: "grok-4-fast-reasoning"},
	})

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	var active CatalogModel
	var retired CatalogModel
	for _, item := range models {
		if item.ID == "grok-4.3" {
			active = item
		}
		if item.ID == "grok-4-fast-reasoning" {
			retired = item
		}
	}

	if active.BestFor == "" {
		t.Fatalf("expected best-for text for active model")
	}
	if retired.RetirementStatus != RetirementStatusRetired {
		t.Fatalf("expected retired status, got %q", retired.RetirementStatus)
	}
	if retired.ReplacementModel != "grok-4.3" {
		t.Fatalf("expected replacement grok-4.3, got %q", retired.ReplacementModel)
	}
}

func TestIsModelRetired(t *testing.T) {
	t.Parallel()

	retired, replacement := IsModelRetired("grok-4-fast-reasoning")
	if !retired {
		t.Fatalf("expected model to be retired")
	}
	if replacement != "grok-4.3" {
		t.Fatalf("unexpected replacement model %q", replacement)
	}
}
