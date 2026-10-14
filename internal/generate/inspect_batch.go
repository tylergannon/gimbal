package generate

import (
	"encoding/json"
	"path/filepath"

	"github.com/tylergannon/polytype/goload"
)

// InspectionRequest selects one source preview; Guide includes its command help.
type InspectionRequest struct {
	Dir   string `json:"dir"`
	Entry string `json:"entry"`
	Name  string `json:"name"`
	Guide bool   `json:"guide"`
}

// InspectionResult keeps a failed preview independent from its healthy siblings.
type InspectionResult struct {
	Name  string          `json:"name"`
	Page  json.RawMessage `json:"page,omitempty"`
	Error string          `json:"error,omitempty"`
}

// InspectBatch analyzes selected previews together. A failing package falls back
// to individual requests so an unrelated broken workflow cannot hide good pages.
func InspectBatch(root string, selected []InspectionRequest, session *goload.Session) []InspectionResult {
	if len(selected) == 0 {
		return []InspectionResult{}
	}
	requests := make([]analysisRequest, len(selected))
	patterns := make([]string, len(selected))
	for i, r := range selected {
		dir := r.Dir
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
		requests[i] = analysisRequest{Dir: dir, Entry: r.Entry, Name: r.Name, Command: r.Guide, Inspect: true}
		patterns[i] = dir
	}
	results, err := analyze(root, nil, patterns, requests, session)
	output := make([]InspectionResult, len(selected))
	for i, r := range selected {
		output[i].Name = r.Name
		var result analysisResult
		currentErr := err
		if err == nil {
			result = results[i]
		} else {
			one, e := analyze(requests[i].Dir, nil, []string{"."}, []analysisRequest{requests[i]}, session)
			currentErr = e
			if e == nil {
				result = one[0]
			}
		}
		if currentErr == nil {
			page := result.Page
			if r.Guide {
				page, currentErr = pageGuide(page, result.Entry.entryInfo(), result.Graph.Roles(), nil)
			}
			if currentErr == nil {
				output[i].Page, currentErr = json.Marshal(page)
			}
		}
		if currentErr != nil {
			output[i].Error = currentErr.Error()
		}
	}
	return output
}
