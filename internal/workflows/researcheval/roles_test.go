package researcheval

import (
	"math"
	"testing"

	"github.com/tylergannon/gimbal/internal/observation"
)

func TestRoleMeasurementsCombinesParallelSessionsWithoutCallingTheirTimeElapsed(t *testing.T) {
	snapshot := observation.RunSnapshot{
		Sessions: map[string]observation.SessionRow{
			"a": {Name: "research-indexing", Model: "configured-alias"},
			"b": {Name: "research-indexing", Model: "configured-alias"},
			"c": {Name: "index-curation"},
		},
		Turns: map[string]observation.TurnRow{
			"a1": {Session: "a", Started: 1000, Ended: 3000, Duration: 2000},
			"b1": {Session: "b", Started: 1000, Ended: 4000, Duration: 3000},
			"a2": {Session: "a", Started: 3000, Ended: 3500, Duration: 500},
			"c1": {Session: "c", Started: 4000, Ended: 5000, Duration: 1000},
		},
		TurnUsage: map[string]map[string]observation.Usage{
			"a1": {"gpt-5.6-luna": {Tokens: observation.Tokens{Input: 1_000_000, CacheRead: 100, CacheWrite: 200, Output: 100_000, Reasoning: 300}}},
			"b1": {"gpt-5.6-luna": {Tokens: observation.Tokens{Input: 2_000_000, CacheRead: 400, CacheWrite: 500, Output: 200_000, Reasoning: 600}}},
			"a2": {"private-model": {Tokens: observation.Tokens{Input: 17}, StatedCost: 0.25}},
			"c1": {"private-model": {Tokens: observation.Tokens{Output: 23}, StatedCost: 0.5}},
		},
	}
	rows := roleMeasurements(snapshot)
	if len(rows) != 2 || rows[0].Role != "index-curation" || rows[1].Role != "research-indexing" {
		t.Fatalf("unexpected roles: %+v", rows)
	}
	research := rows[1]
	if research.Sessions != 2 || research.Turns != 3 || research.SumTurnSeconds != 5.5 {
		t.Fatalf("parallel workload: %+v", research)
	}
	want := observation.Tokens{Input: 3_000_000, CacheRead: 500, CacheWrite: 700, Output: 300_000, Reasoning: 900}
	if research.ByModel["gpt-5.6-luna"].Tokens != want || len(research.ByModel) != 2 {
		t.Fatalf("actual model usage: %+v", research.ByModel)
	}
	priced, known := observation.TotalCost(observation.Total{ByModel: map[string]observation.Usage{"gpt-5.6-luna": {Tokens: want}}})
	if !known || !research.CostKnown || math.Abs(research.CostUSD-(priced+0.25)) > 1e-12 {
		t.Fatalf("research cost: %+v", research)
	}
	if !rows[0].CostKnown || rows[0].CostUSD != 0.5 || rows[0].ByModel["private-model"].Output != 23 {
		t.Fatalf("curation mixed into research: %+v", rows[0])
	}
}

func TestRoleMeasurementsMissingUsageAndUnpricedTurnsStayUnknown(t *testing.T) {
	for _, missing := range []map[string]observation.Usage{
		nil,
		{"private-model": {Tokens: observation.Tokens{Input: 100}}},
	} {
		snapshot := observation.RunSnapshot{
			Sessions: map[string]observation.SessionRow{"s": {Name: "research-indexing"}, "idle": {Name: "unused-role"}},
			Turns: map[string]observation.TurnRow{
				"priced":  {Session: "s", Duration: 1000},
				"missing": {Session: "s", Duration: 250},
			},
			TurnUsage: map[string]map[string]observation.Usage{
				"priced":  {"private-model": {Tokens: observation.Tokens{Input: 200}, StatedCost: 0.1}},
				"missing": missing,
			},
		}
		rows := roleMeasurements(snapshot)
		if rows[0].CostKnown || rows[0].CostUSD != 0.1 || rows[0].Turns != 2 || rows[0].SumTurnSeconds != 1.25 {
			t.Fatalf("missing usage/price masked by another turn: %+v", rows[0])
		}
		if rows[1].CostKnown || rows[1].Sessions != 1 || rows[1].Turns != 0 {
			t.Fatalf("unobserved role claimed measured cost: %+v", rows[1])
		}
	}
}
