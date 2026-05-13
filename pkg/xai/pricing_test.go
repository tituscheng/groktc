package xai

import "testing"

func TestLookupPricing(t *testing.T) {
	t.Parallel()

	pricing, err := LookupPricing("grok-4.3")
	if err != nil {
		t.Fatalf("LookupPricing returned error: %v", err)
	}
	if pricing.InputPerMillionUSD != 1.25 {
		t.Fatalf("unexpected price: %f", pricing.InputPerMillionUSD)
	}
	if pricing.OutputPerMillionUSD != 2.50 {
		t.Fatalf("unexpected output price: %f", pricing.OutputPerMillionUSD)
	}
}

func TestEstimateInputCostUSD(t *testing.T) {
	t.Parallel()

	got := EstimateInputCostUSD(2000, Pricing{InputPerMillionUSD: 1.25})
	want := 0.0025
	if got != want {
		t.Fatalf("expected %f, got %f", want, got)
	}
}

func TestEstimateOutputCostUSD(t *testing.T) {
	t.Parallel()

	got := EstimateOutputCostUSD(1000, Pricing{OutputPerMillionUSD: 2.50})
	want := 0.0025
	if got != want {
		t.Fatalf("expected %f, got %f", want, got)
	}
}
