package researcheval

import (
	"sort"

	"github.com/tylergannon/gimbal/internal/observation"
)

type roleMeasurement struct {
	Role           string                       `json:"role"`
	Sessions       int                          `json:"sessions"`
	Turns          int                          `json:"turns"`
	ByModel        map[string]observation.Usage `json:"by_model"`
	CostUSD        float64                      `json:"cost_usd"`
	CostKnown      bool                         `json:"cost_known"`
	SumTurnSeconds float64                      `json:"sum_turn_seconds"`
}

// roleMeasurements preserves actual session roles and reported model usage.
// SumTurnSeconds adds observed turn durations, including overlapping turns;
// it measures occupied agent time, not elapsed workflow time.
func roleMeasurements(snapshot observation.RunSnapshot) []roleMeasurement {
	roles := map[string]*roleMeasurement{}
	for _, session := range snapshot.Sessions {
		row := roles[session.Name]
		if row == nil {
			row = &roleMeasurement{Role: session.Name, ByModel: map[string]observation.Usage{}, CostKnown: true}
			roles[session.Name] = row
		}
		row.Sessions++
	}
	for id, turn := range snapshot.Turns {
		session, ok := snapshot.Sessions[turn.Session]
		if !ok {
			continue
		}
		row := roles[session.Name]
		row.Turns++
		row.SumTurnSeconds += float64(turn.Duration) / 1000
		if len(snapshot.TurnUsage[id]) == 0 {
			row.CostKnown = false
		}
		for model, usage := range snapshot.TurnUsage[id] {
			sum := row.ByModel[model]
			sum.Input += usage.Input
			sum.CacheRead += usage.CacheRead
			sum.CacheWrite += usage.CacheWrite
			sum.Output += usage.Output
			sum.Reasoning += usage.Reasoning
			sum.StatedCost += usage.StatedCost
			row.ByModel[model] = sum
			cost, known := observation.TotalCost(observation.Total{ByModel: map[string]observation.Usage{model: usage}})
			row.CostUSD += cost
			row.CostKnown = row.CostKnown && known
		}
	}
	result := make([]roleMeasurement, 0, len(roles))
	for _, row := range roles {
		if row.Turns == 0 {
			row.CostKnown = false
		}
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Role < result[j].Role })
	return result
}
