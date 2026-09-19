package observation

import (
	"math"
	"testing"
)

func TestModelPriceFor(t *testing.T) {
	for _, want := range []struct {
		id                                   string
		input, output, cacheRead, cacheWrite float64
	}{
		{"claude-haiku-4-5-20251001", 1, 5, 0.1, 1.25},
		{"claude-fable-5-1", 10, 50, 0.25, 12.5},
		{"claude-opus-5", 5, 25, 0.5, 6.25},
		{"gpt-5.6-luna", 0.2, 1.2, 0.02, 0.25},
		{"gpt-5.6-sol", 4, 20, 0.4, 5},
		{"gemini-3.8-flash", 0.75, 3.75, 0.075, 0},
	} {
		got, ok := modelPriceFor(want.id)
		if !ok || got.input != want.input || got.output != want.output || got.cacheRead != want.cacheRead || got.cacheWrite != want.cacheWrite {
			t.Errorf("price for %s = %#v, %t", want.id, got, ok)
		}
	}
	stripped, ok := modelPriceFor("gemini-3.8-flash-low")
	if !ok || stripped.id != "gemini-3.8-flash" {
		t.Fatalf("stripped lookup = %#v, %t", stripped, ok)
	}
	if _, ok := modelPriceFor("not-a-priced-model"); ok {
		t.Fatal("unknown model has a price")
	}
}

func TestPricedCost(t *testing.T) {
	cost, ok := pricedCost("gpt-5.6-luna", Tokens{
		Input: 1_000_000, CacheRead: 1_000_000, CacheWrite: 1_000_000,
		Output: 1_000_000, Reasoning: 1_000_000,
	})
	if !ok || cost != 2.87 {
		t.Fatalf("priced cost = %v, %t; want 2.87, true", cost, ok)
	}
	if cost, ok := pricedCost("not-a-priced-model", Tokens{}); ok || cost != 0 {
		t.Fatalf("unknown price = %v, %t; want 0, false", cost, ok)
	}
}

func TestTotalCostUsesCatalogAndStatedFallback(t *testing.T) {
	total := Total{ByModel: map[string]Usage{
		"gpt-5.6-luna":  {Tokens: Tokens{Input: 1_000_000, Output: 100_000}},
		"private-model": {StatedCost: 0.5},
	}}
	cost, ok := TotalCost(total)
	if !ok || math.Abs(cost-0.82) > 1e-12 {
		t.Fatalf("total cost = %v, %t; want 0.82, true", cost, ok)
	}

	total.ByModel["unpriced-model"] = Usage{Input: 1}
	if cost, ok := TotalCost(total); ok || cost != 0 {
		t.Fatalf("incomplete total = %v, %t; want 0, false", cost, ok)
	}
}
