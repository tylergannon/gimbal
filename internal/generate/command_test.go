package generate

import (
	"strings"
	"testing"
)

func TestGeneratedDefinitionUsesConcreteInput(t *testing.T) {
	graph, info, err := extract("testdata/fixture", "Fixture", "fixture", nil)
	if err != nil {
		t.Fatal(err)
	}
	page, err := pageGuide(sourcePage{Name: "fixture"}, info, graph.Roles(), map[string]string{"lead": "gpt-5.6-luna"})
	if err != nil {
		t.Fatal(err)
	}
	text := definitionSource("fixture", "Fixture", info.params, info.fields, description(info, graph, page, nil), "Graph", "")
	for _, want := range []string{"gimbal.WorkflowDefinition", "gimbal.ValidateWorkflowInput", "Fixture(ctx,env", "Graph:Graph"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, forbidden := range []string{"func init()", "RegisterGraph", "adapters["} {
		if strings.Contains(text, forbidden) {
			t.Errorf("retains %q", forbidden)
		}
	}
}
