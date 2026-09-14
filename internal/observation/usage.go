package observation

import "strings"

// Total is one roll-up: across every model, and per model.
type Total struct {
	All     Usage            `json:"all"`
	ByModel map[string]Usage `json:"by_model"`
}

// Totals is every scope's and every session's roll-up, recomputed in Go
// whenever a turn's usage changes. The browser never sums anything.
type Totals struct {
	Scopes   map[string]Total `json:"scopes"`
	Sessions map[string]Total `json:"sessions"`
}

// contains reports whether a scope key covers the scope a turn ran in. Scope
// keys are slash paths, so the root "" covers everything and attempt.1 does
// not cover attempt.10.
func contains(scope, turnScope string) bool {
	return scope == "" || turnScope == scope || strings.HasPrefix(turnScope, scope+"/")
}

// totalsLocked recomputes every roll-up from turn_usage. Tens of scopes by a
// few hundred turns: there is no index and nothing is stored.
func (s *Store) totalsLocked() Totals {
	out := Totals{
		Scopes:   make(map[string]Total, len(s.scopes)),
		Sessions: make(map[string]Total, len(s.sessions)),
	}
	for key := range s.scopes {
		out.Scopes[key] = s.totalLocked(func(turn *TurnRow) bool { return contains(key, turn.Scope) })
	}
	for id := range s.sessions {
		out.Sessions[id] = s.totalLocked(func(turn *TurnRow) bool { return turn.Session == id })
	}
	return out
}

// totalLocked sums turn_usage over the turns a predicate picks.
func (s *Store) totalLocked(match func(*TurnRow) bool) Total {
	total := Total{ByModel: map[string]Usage{}}
	for id, turn := range s.turns {
		if !match(turn) {
			continue
		}
		for model, usage := range s.turnUsage[id] {
			total.All = total.All.add(usage)
			total.ByModel[model] = total.ByModel[model].add(usage)
		}
	}
	return total
}
