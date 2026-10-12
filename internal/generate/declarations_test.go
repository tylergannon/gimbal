package generate_test

import (
	"reflect"
	"testing"

	"github.com/tylergannon/gimbal/internal/generate"
	"github.com/tylergannon/gimbal/workflow"
)

func TestGraphValueDeclarations(t *testing.T) {
	graph, err := generate.Extract("testdata/fixture", "DeclarationShape", "declarations")
	if err != nil {
		t.Fatal(err)
	}
	if len(graph.Diagnostics) > 0 {
		t.Fatalf("diagnostics: %v", graph.Diagnostics)
	}
	requirements := []workflow.StoreDeclaration{{Store: "credentials", Names: []string{"TOKEN"}}, {Store: "", Names: []string{"DATABASE_URL"}}}
	service := graph.Services[0]
	if !reflect.DeepEqual(service.Requirements, requirements) || !reflect.DeepEqual(service.Yields, []workflow.StoreDeclaration{{Names: []string{"WEB_URL"}}}) {
		t.Fatalf("service declarations: %+v", service)
	}
	producer := graph.Body[0].(workflow.Command)
	if !reflect.DeepEqual(producer.Requirements, requirements) || !reflect.DeepEqual(producer.Yields, []workflow.StoreDeclaration{{Store: "report", Names: []string{"RESULT"}}}) {
		t.Fatalf("producer declarations: %+v", producer)
	}
	unrelated := graph.Body[1].(workflow.Command)
	if len(unrelated.Requirements) != 0 || unrelated.Yields != nil {
		t.Fatalf("unrelated context acquired declarations: %+v", unrelated)
	}
	helper := graph.Body[2].(workflow.Command)
	if !reflect.DeepEqual(helper.Requirements, requirements) || helper.Yields != nil {
		t.Fatalf("helper declarations: %+v", helper)
	}
	scope := graph.Body[3].(workflow.Scope)
	group := scope.Body[0].(workflow.Group)
	consumer := group.Children[0].Body[1].(workflow.AgentCall)
	expected := append(requirements, workflow.StoreDeclaration{Store: "child", Names: []string{"EXTRA"}})
	if !reflect.DeepEqual(consumer.Requirements, expected) {
		t.Fatalf("nested group consumer: %+v", consumer.Requirements)
	}
}

func TestServiceCheckGraphValueDeclarations(t *testing.T) {
	graph, err := generate.Extract("../workflows/servicecheck", "ServiceCheck", "service-check")
	if err != nil {
		t.Fatal(err)
	}
	scope := graph.Body[1].(workflow.Scope)
	if len(scope.Services) != 2 {
		t.Fatalf("services: %+v", scope.Services)
	}
	for i, name := range []string{"DATABASE_URL", "WEB_URL"} {
		yields := scope.Services[i].Yields
		if yields == nil || len(yields) != 1 || yields[0].Store != "" || !reflect.DeepEqual(yields[0].Names, []string{name}) {
			t.Fatalf("service %s yields: %+v", scope.Services[i].Name, yields)
		}
	}
	if len(scope.Services[1].Requirements) != 1 || scope.Services[1].Requirements[0].Names[0] != "DATABASE_URL" {
		t.Fatalf("web requirements: %+v", scope.Services[1].Requirements)
	}
}
