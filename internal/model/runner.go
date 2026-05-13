package model

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tituscheng/groktc/internal/config"
	"github.com/tituscheng/groktc/pkg/xai"
)

const envAPIKey = "XAI_API_KEY"

type RunnerOptions struct {
	SetModelID string
	ListOnly   bool
}

type Runner struct {
	Client          xai.LanguageModelCatalogClient
	StoreFactory    func() (Store, error)
	Stdout          io.Writer
	Stderr          io.Writer
	Stdin           io.Reader
	LookupEnv       func(string) string
	TerminalChecker func() bool
	Prompt          func(io.Reader, io.Writer, []CatalogModel, string) (string, bool, error)
}

func NewRunner() *Runner {
	return &Runner{
		Client: xai.NewClient(os.Getenv(envAPIKey)),
		StoreFactory: func() (Store, error) {
			return OpenDefaultStore(os.UserHomeDir)
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		LookupEnv: func(key string) string {
			return os.Getenv(key)
		},
		TerminalChecker: isTerminal,
		Prompt:          promptModelSelection,
	}
}

func (r *Runner) Run(ctx context.Context, opts RunnerOptions) error {
	store, err := r.StoreFactory()
	if err != nil {
		return err
	}
	defer store.Close()

	selectedModel, err := store.GetSelectedModel(ctx)
	if err != nil {
		return err
	}

	models, source, err := r.loadModels(ctx, store)
	if err != nil {
		return err
	}

	if opts.SetModelID != "" {
		return r.setModel(ctx, store, models, opts.SetModelID)
	}

	if opts.ListOnly || !r.TerminalChecker() {
		r.renderList(models, selectedModel, source)
		return nil
	}

	chosenModelID, canceled, err := r.Prompt(r.Stdin, r.Stdout, models, selectedModel)
	if err != nil {
		return err
	}
	if canceled {
		return fmt.Errorf("model selection canceled")
	}

	return r.setModel(ctx, store, models, chosenModelID)
}

func (r *Runner) loadModels(ctx context.Context, store Store) ([]CatalogModel, string, error) {
	apiKey := strings.TrimSpace(r.LookupEnv(envAPIKey))
	if apiKey != "" {
		liveModels, err := r.Client.ListLanguageModels(ctx)
		if err == nil {
			models := make([]CatalogModel, 0, len(liveModels))
			now := time.Now().UTC()
			for _, item := range liveModels {
				models = append(models, CatalogModel{
					ID:                          item.ID,
					Aliases:                     item.Aliases,
					InputModalities:             item.InputModalities,
					OutputModalities:            item.OutputModalities,
					Version:                     item.Version,
					PromptTextTokenPriceRaw:     item.PromptTextTokenPriceRaw,
					CompletionTextTokenPriceRaw: item.CompletionTextTokenPriceRaw,
					LastFetchedAt:               now,
					RawJSON:                     item.RawJSON,
				})
			}
			models = EnrichModels(models)
			models = filterRetiredModels(models)

			if upsertErr := store.UpsertModelCache(ctx, models); upsertErr != nil {
				return nil, "", fmt.Errorf("cache live models: %w", upsertErr)
			}
			return models, "live API", nil
		}
	}

	cached, err := store.ListModelCache(ctx)
	if err != nil {
		return nil, "", err
	}
	if len(cached) == 0 {
		if apiKey == "" {
			return nil, "", fmt.Errorf("%s is not set and no cached models were found in %s", envAPIKey, filepath.Join(config.DirectoryName, config.DatabaseName))
		}
		return nil, "", fmt.Errorf("could not load live model catalog and no cached models are available")
	}

	return filterRetiredModels(EnrichModels(cached)), "cache", nil
}

func (r *Runner) renderList(models []CatalogModel, selectedModel string, source string) {
	current := strings.TrimSpace(selectedModel)
	if current == "" {
		current = DefaultModelID
	}

	fmt.Fprintf(r.Stdout, "%s: %s\n", colorHeader("Model catalog source"), source)
	fmt.Fprintf(r.Stdout, "%s: %s\n", colorHeader("Current model"), colorCurrent(current))
	fmt.Fprintf(r.Stdout, "%s: %s\n\n", colorHeader("Default fallback"), DefaultModelID)

	tw := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, colorHeader("CURRENT")+"\t"+colorHeader("MODEL")+"\t"+colorHeader("PRICING")+"\t"+colorHeader("BEST FOR")+"\t"+colorHeader("STATUS"))
	for _, item := range models {
		marker := ""
		if item.ID == current {
			marker = colorCurrent("*")
		}
		status := item.RetirementLabel()
		if status == "" {
			status = colorMuted("-")
		} else {
			status = colorWarn(status)
		}
		bestFor := item.BestFor
		if bestFor == "" {
			bestFor = colorMuted("-")
		}
		price := formatPriceSummary(item)
		if price == "" {
			price = colorMuted("-")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", marker, colorModel(item.ID), price, bestFor, status)
	}
	_ = tw.Flush()
}

func (r *Runner) setModel(ctx context.Context, store Store, models []CatalogModel, modelID string) error {
	candidate := strings.TrimSpace(modelID)
	if candidate == "" {
		return fmt.Errorf("model id is required")
	}

	foundModel, found := findModelByID(models, candidate)
	if !found {
		return fmt.Errorf("model %q is not available in the catalog", candidate)
	}

	if foundModel.IsRetired() || foundModel.IsRetiring() {
		label := foundModel.RetirementLabel()
		if label == "" {
			label = "retired"
		}
		return fmt.Errorf("model %q cannot be selected: %s", foundModel.ID, label)
	}

	if err := store.SetSelectedModel(ctx, foundModel.ID); err != nil {
		return err
	}

	fmt.Fprintf(r.Stdout, "%s: %s\n", colorHeader("Selected model"), colorModel(foundModel.ID))
	if foundModel.BestFor != "" {
		fmt.Fprintf(r.Stdout, "%s: %s\n", colorHeader("Best for"), foundModel.BestFor)
	}
	return nil
}

func findModelByID(models []CatalogModel, modelID string) (CatalogModel, bool) {
	for _, item := range models {
		if item.ID == modelID {
			return item, true
		}
		for _, alias := range item.Aliases {
			if alias == modelID {
				return item, true
			}
		}
	}
	return CatalogModel{}, false
}

func filterRetiredModels(models []CatalogModel) []CatalogModel {
	filtered := make([]CatalogModel, 0, len(models))
	for _, item := range models {
		if item.IsRetired() {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
