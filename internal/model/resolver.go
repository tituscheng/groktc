package model

import (
	"context"
	"fmt"
	"strings"
)

type Resolver struct {
	Store        Store
	DefaultModel string
}

type ResolveResult struct {
	ModelID  string
	Warnings []string
}

func (r Resolver) ResolveForTokenize(ctx context.Context, model string, explicit bool) (ResolveResult, error) {
	defaultModel := r.DefaultModel
	if strings.TrimSpace(defaultModel) == "" {
		defaultModel = DefaultModelID
	}

	requested := strings.TrimSpace(model)
	if explicit {
		if retired, replacement := IsModelRetired(requested); retired {
			if replacement == "" {
				return ResolveResult{}, fmt.Errorf("model %q is retired", requested)
			}
			return ResolveResult{}, fmt.Errorf("model %q is retired; use %q", requested, replacement)
		}
		return ResolveResult{ModelID: requested}, nil
	}

	if r.Store == nil {
		return ResolveResult{ModelID: defaultModel}, nil
	}

	selected, err := r.Store.GetSelectedModel(ctx)
	if err != nil {
		return ResolveResult{ModelID: defaultModel, Warnings: []string{fmt.Sprintf("Could not read selected model from store: %v", err)}}, nil
	}
	if strings.TrimSpace(selected) == "" {
		return ResolveResult{ModelID: defaultModel}, nil
	}

	if retired, replacement := IsModelRetired(selected); retired {
		warning := fmt.Sprintf("Selected model %q is retired; falling back to %q", selected, defaultModel)
		if replacement != "" {
			warning = fmt.Sprintf("Selected model %q is retired; use %q. Falling back to %q", selected, replacement, defaultModel)
		}
		return ResolveResult{
			ModelID:  defaultModel,
			Warnings: []string{warning},
		}, nil
	}

	_, found, err := r.Store.GetModelByID(ctx, selected)
	if err == nil && !found {
		return ResolveResult{
			ModelID:  defaultModel,
			Warnings: []string{fmt.Sprintf("Selected model %q is not in cached model list; falling back to %q", selected, defaultModel)},
		}, nil
	}
	if err != nil {
		return ResolveResult{
			ModelID:  defaultModel,
			Warnings: []string{fmt.Sprintf("Could not validate selected model %q in cache: %v. Falling back to %q", selected, err, defaultModel)},
		}, nil
	}

	return ResolveResult{ModelID: selected}, nil
}
