package workflows

import (
	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/skgo"
)

type WorkflowItem struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
}
type WorkflowList struct {
	Workflows []WorkflowItem `json:"workflows"`
}

func load(event PageRequestEvent) (WorkflowList, error) {
	result := WorkflowList{Workflows: []WorkflowItem{}}
	for _, d := range gimbal.DefaultRegistry.Catalog() {
		result.Workflows = append(result.Workflows, WorkflowItem{Name: d.Name, Summary: d.Summary})
	}
	return result, nil
}

var _ = skgo.Load(load)
