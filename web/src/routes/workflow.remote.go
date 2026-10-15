package routes

import (
	"context"
	"slices"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/skgo"
)

type WorkflowField struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Optional    bool   `json:"optional"`
	Nullable    bool   `json:"nullable"`
}
type WorkflowRole struct {
	Name    string `json:"name"`
	Default string `json:"default"`
}
type WorkflowDetails struct {
	Name       string          `json:"name"`
	Summary    string          `json:"summary"`
	Help       string          `json:"help"`
	Fields     []WorkflowField `json:"fields"`
	Roles      []WorkflowRole  `json:"roles"`
	SourceJSON string          `json:"source_json"`
}
type WorkflowCatalog struct {
	Project   string            `json:"project"`
	Workflows []WorkflowDetails `json:"workflows"`
}

func describeWorkflow(name string) (WorkflowDetails, error) {
	d, ok := gimbal.DefaultRegistry.Workflow(name)
	if !ok {
		return WorkflowDetails{}, skgo.Errorf(404, "Unknown workflow")
	}
	result := WorkflowDetails{Name: d.Name, Summary: d.Summary, Help: d.Help, Fields: []WorkflowField{}, Roles: []WorkflowRole{}, SourceJSON: string(d.SourcePage)}
	for _, field := range d.Fields {
		result.Fields = append(result.Fields, WorkflowField{Name: field.Name, Kind: field.Kind, Description: field.Description, Optional: field.Optional, Nullable: field.Nullable})
	}
	for role, model := range d.Roles {
		result.Roles = append(result.Roles, WorkflowRole{Name: string(role), Default: model})
	}
	slices.SortFunc(result.Roles, func(a, b WorkflowRole) int { return strings.Compare(a.Name, b.Name) })
	return result, nil
}

func workflowCatalog(ctx context.Context, projectID string) (WorkflowCatalog, error) {
	owner := host.OwnerFrom(ctx)
	if owner == nil {
		return WorkflowCatalog{}, skgo.Errorf(503, "Workflow host unavailable")
	}
	result := WorkflowCatalog{Workflows: []WorkflowDetails{}}
	for _, project := range owner.Projects() {
		if project.ID() == projectID {
			result.Project = project.Path()
			break
		}
	}
	if result.Project == "" {
		return WorkflowCatalog{}, skgo.Errorf(404, "Project not found")
	}
	for _, item := range gimbal.DefaultRegistry.Catalog() {
		detail, err := describeWorkflow(item.Name)
		if err != nil {
			return WorkflowCatalog{}, err
		}
		result.Workflows = append(result.Workflows, detail)
	}
	return result, nil
}
func workflowGuide(_ context.Context, name string) (WorkflowDetails, error) {
	return describeWorkflow(name)
}

var _ = skgo.Query(workflowCatalog)
var _ = skgo.Query(workflowGuide)
