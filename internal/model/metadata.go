package model

import "strings"

type retirementInfo struct {
	Status      RetirementStatus
	Replacement string
}

var retirementByModel = map[string]retirementInfo{
	"grok-4-1-fast-reasoning":     {Status: RetirementStatusRetired, Replacement: "grok-4.3"},
	"grok-4-1-fast-non-reasoning": {Status: RetirementStatusRetired, Replacement: "grok-4.20-non-reasoning"},
	"grok-4-fast-reasoning":       {Status: RetirementStatusRetired, Replacement: "grok-4.3"},
	"grok-4-fast-non-reasoning":   {Status: RetirementStatusRetired, Replacement: "grok-4.20-non-reasoning"},
	"grok-4-0709":                 {Status: RetirementStatusRetired, Replacement: "grok-4.3"},
	"grok-code-fast-1":            {Status: RetirementStatusRetired, Replacement: "grok-4.3"},
	"grok-3":                      {Status: RetirementStatusRetired, Replacement: "grok-4.3"},
}

var bestForByModel = map[string]string{
	"grok-4.3":                  "best for high-quality reasoning, coding, and tool-calling workflows",
	"grok-4.20-reasoning":       "best for strong reasoning tasks with explicit reasoning mode",
	"grok-4.20-non-reasoning":   "best for fast, lower-latency non-reasoning text generation",
	"grok-4":                    "best for general-purpose text tasks with stable model alias behavior",
	"grok-4-latest":             "best for adopting latest grok-4 improvements automatically",
	"grok-code-fast-1":          "best for fast coding tasks (retired; migrate to grok-4.3)",
	"grok-4-fast-reasoning":     "best for fast reasoning requests (retired; migrate to grok-4.3)",
	"grok-4-fast-non-reasoning": "best for low-latency text generation (retired; migrate to grok-4.20-non-reasoning)",
}

func EnrichModels(models []CatalogModel) []CatalogModel {
	updated := make([]CatalogModel, 0, len(models))
	for _, item := range models {
		item.BestFor = bestForText(item)
		status, replacement := retirementFor(item)
		item.RetirementStatus = status
		item.ReplacementModel = replacement
		updated = append(updated, item)
	}
	SortModels(updated)
	return updated
}

func IsModelRetired(modelID string) (bool, string) {
	info, ok := retirementByModel[NormalizeModelID(modelID)]
	if !ok {
		return false, ""
	}
	return info.Status == RetirementStatusRetired, info.Replacement
}

func bestForText(item CatalogModel) string {
	keys := append([]string{item.ID}, item.Aliases...)
	for _, key := range keys {
		if text, ok := bestForByModel[NormalizeModelID(key)]; ok {
			return text
		}
	}

	modelKey := NormalizeModelID(item.ID)
	if strings.Contains(modelKey, "reasoning") {
		return "best for complex reasoning and multi-step problem solving"
	}
	if strings.Contains(modelKey, "non-reasoning") {
		return "best for low-latency text generation and high-throughput requests"
	}
	if strings.Contains(modelKey, "code") {
		return "best for programming and code generation tasks"
	}
	if hasModality(item.InputModalities, "image") {
		return "best for mixed text and image understanding tasks"
	}
	return "best for general-purpose language tasks"
}

func retirementFor(item CatalogModel) (RetirementStatus, string) {
	keys := append([]string{item.ID}, item.Aliases...)
	for _, key := range keys {
		if info, ok := retirementByModel[NormalizeModelID(key)]; ok {
			return info.Status, info.Replacement
		}
	}
	return RetirementStatusNone, ""
}

func hasModality(modalities []string, target string) bool {
	for _, modality := range modalities {
		if strings.EqualFold(modality, target) {
			return true
		}
	}
	return false
}
