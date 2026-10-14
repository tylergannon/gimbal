package generate

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/tylergannon/gimbal/workflow"
	"github.com/tylergannon/polytype/goload"
)

type analysisRequest struct {
	Dir, Entry, Name string
	Command, Inspect bool
}

// analysisResult contains only Gimbal's extracted products. In particular the
// inspector is projected before storage: its intermediate maps use AST/type
// identities and private service markers, which do not survive serialization.
type analysisResult struct {
	Graph workflow.Graph
	Page  sourcePage
	Entry cachedEntry
}

type cachedEntry struct {
	Params, Summary, Long string
	Fields                []cachedField
}
type cachedField struct {
	Name, Flag, Kind, Doc, JSON string
	Optional                    bool
}

func cacheEntry(info entryInfo) cachedEntry {
	result := cachedEntry{Params: info.params, Summary: info.summary, Long: info.long}
	for _, f := range info.fields {
		result.Fields = append(result.Fields, cachedField{f.name, f.flag, f.kind, f.doc, f.json, f.optional})
	}
	return result
}
func (info cachedEntry) entryInfo() entryInfo {
	result := entryInfo{params: info.Params, summary: info.Summary, long: info.Long}
	for _, f := range info.Fields {
		result.fields = append(result.fields, field{f.Name, f.Flag, f.Kind, f.Doc, f.JSON, f.Optional})
	}
	return result
}

func analyze(dir string, overlay map[string][]byte, patterns []string, requests []analysisRequest, session *goload.Session) ([]analysisResult, error) {
	for i := range requests {
		absolute, err := filepath.Abs(requests[i].Dir)
		if err != nil {
			return nil, err
		}
		requests[i].Dir = absolute
	}
	cache := openAnalysisCache()
	var input *analysisInputs
	if cache != nil {
		var captureErr error
		input, captureErr = captureAnalysisInputs(dir, overlay, patterns)
		var packageErr *packageInputError
		if errors.As(captureErr, &packageErr) {
			return nil, packageErr
		}
		if input != nil {
			if result, ok := cache.read(input, requests); ok {
				return result, nil
			}
		}
	}
	var loaded *sourcePackages
	var err error
	if input != nil {
		loaded, err = input.load(session)
	} else {
		loaded, err = loadSourceWithSession(dir, overlay, session, patterns...)
	}
	if err != nil {
		return nil, err
	}
	var result []analysisResult
	for _, request := range requests {
		pkg, err := loaded.packageAt(request.Dir)
		if err != nil {
			return nil, err
		}
		graph, info, err := extractLoaded(pkg, loaded.imports, request.Entry, request.Name, request.Command, nil)
		if err != nil {
			return nil, err
		}
		item := analysisResult{Graph: graph, Entry: cacheEntry(info)}
		if request.Inspect {
			details := &inspectionData{}
			inspected, _, err := extractLoaded(pkg, loaded.imports, request.Entry, request.Name, request.Command, details)
			if err != nil {
				return nil, err
			}
			item.Page = inspectGraph(inspected, request.Entry, details)
		}
		result = append(result, item)
	}
	if cache != nil && input != nil && input.bound {
		cache.write(input, requests, result)
	}
	return result, nil
}

func analyzeOne(dir, entry, name string, overlay map[string][]byte, command, inspect bool) (analysisResult, error) {
	results, err := analyze(dir, overlay, []string{"."}, []analysisRequest{{dir, entry, name, command, inspect}}, nil)
	if err != nil {
		return analysisResult{}, err
	}
	if len(results) != 1 {
		return analysisResult{}, fmt.Errorf("generate: expected one analysis result")
	}
	return results[0], nil
}
