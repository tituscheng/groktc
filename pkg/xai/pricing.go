package xai

import "fmt"

type Pricing struct {
	InputPerMillionUSD  float64
	OutputPerMillionUSD float64
}

var pricingByModel = map[string]Pricing{
	"grok-4.3": {
		InputPerMillionUSD:  1.25,
		OutputPerMillionUSD: 2.50,
	},
}

func LookupPricing(model string) (Pricing, error) {
	pricing, ok := pricingByModel[model]
	if !ok {
		return Pricing{}, fmt.Errorf("no pricing is configured for model %q", model)
	}

	return pricing, nil
}

func EstimateInputCostUSD(tokens int, pricing Pricing) float64 {
	return (float64(tokens) / 1_000_000) * pricing.InputPerMillionUSD
}

func EstimateOutputCostUSD(tokens int, pricing Pricing) float64 {
	return (float64(tokens) / 1_000_000) * pricing.OutputPerMillionUSD
}
