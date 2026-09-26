package session

import (
	"testing"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

func TestUsageTotalsAdd(t *testing.T) {
	totals := UsageTotals{}
	totals.AddUsage(model.Usage{Input: 1, Output: 2, CacheRead: 3, CacheWrite: 4, Cost: model.CostBreakdown{Total: 0.5}})
	totals.AddUsage(model.Usage{Input: 10, Output: 20, CacheRead: 30, CacheWrite: 40, Cost: model.CostBreakdown{Total: 1.5}})
	if totals.Input != 11 || totals.Output != 22 || totals.CacheRead != 33 || totals.CacheWrite != 44 {
		t.Fatalf("totals = %+v", totals)
	}
	if totals.Cost != 2.0 {
		t.Fatalf("cost = %v, want 2.0", totals.Cost)
	}
}

func TestUsageCostBreakdown(t *testing.T) {
	entries := []model.SessionEntry{
		&model.SessionMessageEntry{Message: &model.AssistantMessage{
			Provider: model.ProviderOpenAI, Model: "mock",
			Usage: model.Usage{Input: 100, Output: 50, Cost: model.CostBreakdown{Total: 0.25}},
		}},
		&model.SessionMessageEntry{Message: &model.AssistantMessage{
			Provider: model.ProviderOpenAI, Model: "other",
			Usage: model.Usage{Input: 10, Output: 5, Cost: model.CostBreakdown{Total: 0.1}},
		}},
		&model.UsageEntry{Provider: "openai", Model: "mock", Usage: model.Usage{Input: 7, Cost: model.CostBreakdown{Total: 0.05}}},
	}
	breakdown := UsageCostBreakdown(entries)
	if len(breakdown) != 2 {
		t.Fatalf("breakdown = %+v", breakdown)
	}
	if breakdown[0].Key != "openai/mock" || breakdown[0].Cost != 0.3 {
		t.Fatalf("first = %+v", breakdown[0])
	}
	if breakdown[0].Tokens != 157 {
		t.Fatalf("first tokens = %d, want 157", breakdown[0].Tokens)
	}
}
