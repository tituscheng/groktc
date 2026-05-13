package model

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseInputUSDPerMillion(raw string) (float64, bool) {
	return parseUSDPerMillion(raw)
}

func ParseOutputUSDPerMillion(raw string) (float64, bool) {
	return parseUSDPerMillion(raw)
}

func parseUSDPerMillion(raw string) (float64, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, false
	}

	// xAI /v1/language-models uses integer prices in USD cents per 100 million tokens.
	if value >= 1000 {
		return value / 10000, true
	}

	// Some payloads may already be expressed in USD per 1M tokens.
	return value, true
}

func formatPriceSummary(model CatalogModel) string {
	inputUSD, okInput := parseUSDPerMillion(model.PromptTextTokenPriceRaw)
	outputUSD, okOutput := parseUSDPerMillion(model.CompletionTextTokenPriceRaw)

	switch {
	case okInput && okOutput:
		return fmt.Sprintf("input $%.4f / 1M, output $%.4f / 1M", inputUSD, outputUSD)
	case okInput:
		return fmt.Sprintf("input $%.4f / 1M", inputUSD)
	case okOutput:
		return fmt.Sprintf("output $%.4f / 1M", outputUSD)
	default:
		return ""
	}
}
