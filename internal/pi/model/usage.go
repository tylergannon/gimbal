package model

// CostBreakdown is the per-bucket dollar cost of a request.
type CostBreakdown struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
	Total      float64 `json:"total"`
}

// Usage holds token counts and cost for a request.
type Usage struct {
	Input      int `json:"input"`
	Output     int `json:"output"`
	CacheRead  int `json:"cacheRead"`
	CacheWrite int `json:"cacheWrite"`
	// CacheWrite1h is the part of the cache write done with 1h retention. Only
	// Anthropic reports this split.
	CacheWrite1h int `json:"cacheWrite1h,omitempty"`
	// Reasoning is the count of reasoning/thinking tokens, a subset of Output.
	Reasoning   int           `json:"reasoning,omitempty"`
	TotalTokens int           `json:"totalTokens"`
	Cost        CostBreakdown `json:"cost"`
}

// Add returns the sum of two usage values.
func (u Usage) Add(other Usage) Usage {
	return Usage{
		Input:        u.Input + other.Input,
		Output:       u.Output + other.Output,
		CacheRead:    u.CacheRead + other.CacheRead,
		CacheWrite:   u.CacheWrite + other.CacheWrite,
		CacheWrite1h: u.CacheWrite1h + other.CacheWrite1h,
		Reasoning:    u.Reasoning + other.Reasoning,
		TotalTokens:  u.TotalTokens + other.TotalTokens,
		Cost: CostBreakdown{
			Input:      u.Cost.Input + other.Cost.Input,
			Output:     u.Cost.Output + other.Cost.Output,
			CacheRead:  u.Cost.CacheRead + other.Cost.CacheRead,
			CacheWrite: u.Cost.CacheWrite + other.Cost.CacheWrite,
			Total:      u.Cost.Total + other.Cost.Total,
		},
	}
}
