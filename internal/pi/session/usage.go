package session

import (
	"sort"

	"github.com/tylergannon/gimbal/internal/pi/model"
)

// This file ports core/usage-totals.ts at upstream pin d6af72e1.

// UsageTotals is the accumulated usage for one bucket.
type UsageTotals struct {
	Input      int
	Output     int
	CacheRead  int
	CacheWrite int
	Cost       float64
}

// AddUsage adds one usage report into the totals.
func (t *UsageTotals) AddUsage(usage model.Usage) {
	t.Input += usage.Input
	t.Output += usage.Output
	t.CacheRead += usage.CacheRead
	t.CacheWrite += usage.CacheWrite
	t.Cost += usage.Cost.Total
}

// UsageCostBreakdownEntry is one model- or summary-attributed usage bucket.
type UsageCostBreakdownEntry struct {
	Key    string
	Cost   float64
	Tokens int
}

// UsageCostBreakdown groups model-attributed usage by model and all other
// usage under "Tools/summaries", dropping empty buckets and sorting by cost.
func UsageCostBreakdown(entries []model.SessionEntry) []UsageCostBreakdownEntry {
	totalsByKey := map[string]*UsageTotals{}

	for _, entry := range entries {
		var key string
		var usage *model.Usage
		switch e := entry.(type) {
		case model.SessionMessageEntry:
			switch message := e.Message.(type) {
			case model.AssistantMessage:
				key = modelKey(string(message.Provider), message.ResponseModel, message.Model)
				usage = &message.Usage
			case *model.AssistantMessage:
				key = modelKey(string(message.Provider), message.ResponseModel, message.Model)
				usage = &message.Usage
			case model.ToolResultMessage:
				if message.Usage != nil {
					key = "Tools/summaries"
					usage = message.Usage
				}
			case *model.ToolResultMessage:
				if message.Usage != nil {
					key = "Tools/summaries"
					usage = message.Usage
				}
			}
		case *model.SessionMessageEntry:
			if e == nil {
				continue
			}
			switch message := e.Message.(type) {
			case model.AssistantMessage:
				key = modelKey(string(message.Provider), message.ResponseModel, message.Model)
				usage = &message.Usage
			case *model.AssistantMessage:
				key = modelKey(string(message.Provider), message.ResponseModel, message.Model)
				usage = &message.Usage
			case model.ToolResultMessage:
				if message.Usage != nil {
					key = "Tools/summaries"
					usage = message.Usage
				}
			case *model.ToolResultMessage:
				if message.Usage != nil {
					key = "Tools/summaries"
					usage = message.Usage
				}
			}
		case model.UsageEntry:
			key = modelKey(e.Provider, "", e.Model)
			usage = &e.Usage
		case *model.UsageEntry:
			if e == nil {
				continue
			}
			key = modelKey(e.Provider, "", e.Model)
			usage = &e.Usage
		case model.CompactionEntry:
			if e.Usage != nil {
				key = "Tools/summaries"
				usage = e.Usage
			}
		case *model.CompactionEntry:
			if e != nil && e.Usage != nil {
				key = "Tools/summaries"
				usage = e.Usage
			}
		case model.BranchSummaryEntry:
			if e.Usage != nil {
				key = "Tools/summaries"
				usage = e.Usage
			}
		case *model.BranchSummaryEntry:
			if e != nil && e.Usage != nil {
				key = "Tools/summaries"
				usage = e.Usage
			}
		}
		if key == "" || usage == nil {
			continue
		}
		totals := totalsByKey[key]
		if totals == nil {
			totals = &UsageTotals{}
			totalsByKey[key] = totals
		}
		totals.AddUsage(*usage)
	}

	result := make([]UsageCostBreakdownEntry, 0, len(totalsByKey))
	for key, totals := range totalsByKey {
		result = append(result, UsageCostBreakdownEntry{
			Key:    key,
			Cost:   totals.Cost,
			Tokens: totals.Input + totals.Output + totals.CacheRead + totals.CacheWrite,
		})
	}
	result = slicesFilter(result, func(entry UsageCostBreakdownEntry) bool {
		return entry.Cost > 0 || entry.Tokens > 0
	})
	sort.SliceStable(result, func(i, j int) bool { return result[i].Cost > result[j].Cost })
	return result
}

func modelKey(provider, responseModel, modelID string) string {
	if responseModel != "" {
		return provider + "/" + responseModel
	}
	return provider + "/" + modelID
}

func slicesFilter[T any](input []T, keep func(T) bool) []T {
	out := input[:0]
	for _, value := range input {
		if keep(value) {
			out = append(out, value)
		}
	}
	return out
}
