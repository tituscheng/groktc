package model

import "testing"

func TestParseInputUSDPerMillionFromCentsPerHundredMillion(t *testing.T) {
	t.Parallel()

	got, ok := ParseInputUSDPerMillion("12500")
	if !ok {
		t.Fatal("expected parse success")
	}
	if got != 1.25 {
		t.Fatalf("expected 1.25, got %f", got)
	}
}

func TestParseInputUSDPerMillionRawUSD(t *testing.T) {
	t.Parallel()

	got, ok := ParseInputUSDPerMillion("1.25")
	if !ok {
		t.Fatal("expected parse success")
	}
	if got != 1.25 {
		t.Fatalf("expected 1.25, got %f", got)
	}
}

func TestParseOutputUSDPerMillionFromCentsPerHundredMillion(t *testing.T) {
	t.Parallel()

	got, ok := ParseOutputUSDPerMillion("25000")
	if !ok {
		t.Fatal("expected parse success")
	}
	if got != 2.5 {
		t.Fatalf("expected 2.5, got %f", got)
	}
}
