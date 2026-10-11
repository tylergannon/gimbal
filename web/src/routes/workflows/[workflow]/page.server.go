package workflowpage

import (
	"net/http"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/skgo"
)

type RunField struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Optional    bool   `json:"optional"`
	Nullable    bool   `json:"nullable"`
}
type RunRole struct {
	Name    string `json:"name"`
	Default string `json:"default"`
}
type RunProject struct {
	Path string `json:"path"`
}
type WorkflowData struct {
	SourceJSON string       `json:"source_json"`
	Name       string       `json:"name"`
	Help       string       `json:"help"`
	Fields     []RunField   `json:"fields"`
	Roles      []RunRole    `json:"roles"`
	Projects   []RunProject `json:"projects"`
}

func load(event PageRequestEvent) (WorkflowData, error) {
	d, ok := gimbal.DefaultRegistry.Workflow(event.Params.Workflow())
	if !ok {
		return WorkflowData{}, skgo.Errorf(http.StatusNotFound, "Unknown workflow")
	}
	result := WorkflowData{SourceJSON: string(d.SourcePage), Name: d.Name, Help: d.Help, Fields: []RunField{}, Roles: []RunRole{}, Projects: []RunProject{}}
	for _, f := range d.Fields {
		result.Fields = append(result.Fields, RunField{Name: f.Name, Kind: f.Kind, Description: f.Description, Optional: f.Optional, Nullable: f.Nullable})
	}
	for name, value := range d.Roles {
		result.Roles = append(result.Roles, RunRole{Name: string(name), Default: value})
	}
	slices.SortFunc(result.Roles, func(a, b RunRole) int { return strings.Compare(a.Name, b.Name) })
	for _, p := range host.Projects(event.Context()) {
		result.Projects = append(result.Projects, RunProject{Path: p.Path})
	}
	return result, nil
}

var _ = skgo.Load(load)
