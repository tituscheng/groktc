package model

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	DefaultModelID          = "grok-4.3"
	SelectedModelSettingKey = "selected_model_id"
)

type RetirementStatus string

const (
	RetirementStatusNone     RetirementStatus = ""
	RetirementStatusRetiring RetirementStatus = "retiring"
	RetirementStatusRetired  RetirementStatus = "retired"
)

type CatalogModel struct {
	ID                          string
	Aliases                     []string
	InputModalities             []string
	OutputModalities            []string
	Version                     string
	PromptTextTokenPriceRaw     string
	CompletionTextTokenPriceRaw string
	BestFor                     string
	RetirementStatus            RetirementStatus
	ReplacementModel            string
	LastFetchedAt               time.Time
	RawJSON                     string
}

func (m CatalogModel) IsRetired() bool {
	return m.RetirementStatus == RetirementStatusRetired
}

func (m CatalogModel) IsRetiring() bool {
	return m.RetirementStatus == RetirementStatusRetiring
}

func (m CatalogModel) RetirementLabel() string {
	switch m.RetirementStatus {
	case RetirementStatusRetiring:
		if m.ReplacementModel != "" {
			return fmt.Sprintf("retiring (use %s)", m.ReplacementModel)
		}
		return "retiring"
	case RetirementStatusRetired:
		if m.ReplacementModel != "" {
			return fmt.Sprintf("retired (use %s)", m.ReplacementModel)
		}
		return "retired"
	default:
		return ""
	}
}

func (m CatalogModel) DisplayAliases() string {
	if len(m.Aliases) == 0 {
		return "-"
	}
	return strings.Join(m.Aliases, ", ")
}

func NormalizeModelID(modelID string) string {
	return strings.TrimSpace(strings.ToLower(modelID))
}

func SortModels(models []CatalogModel) {
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})
}
