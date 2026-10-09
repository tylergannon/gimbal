package generate

import (
	"testing"

	"github.com/tylergannon/gimbal/workflow"
)

func keyedFixture(body []workflow.Operation) sourcePage {
	return inspectGraph(workflow.Graph{
		Name: "Keys", Source: workflow.Source{File: "fixture.go", Line: 1}, Body: body,
	}, "Keys", &inspectionData{Calls: map[workflow.Source][]callInspection{}})
}

func findNode(nodes []*viewNode, label string) *viewNode {
	for _, node := range nodes {
		if node.Label == label {
			return node
		}
		for _, children := range node.Children {
			if found := findNode(children, label); found != nil {
				return found
			}
		}
	}
	return nil
}

func TestPresentationKeysDeterministicAndIndependentOfLayoutIDs(t *testing.T) {
	body := []workflow.Operation{
		workflow.Set{Source: workflow.Source{File: "fixture.go", Line: 10}, Key: "target"},
		workflow.Condition{Source: workflow.Source{File: "fixture.go", Line: 20}, Branches: []workflow.Branch{
			{Source: workflow.Source{File: "fixture.go", Line: 21}, Case: "ready", Body: []workflow.Operation{
				workflow.Set{Source: workflow.Source{File: "fixture.go", Line: 22}, Key: "nested"},
			}},
		}},
	}
	first := keyedFixture(body)
	second := keyedFixture(body)
	targetA, targetB := findNode(first.Body, "target"), findNode(second.Body, "target")
	if targetA == nil || targetB == nil || targetA.Key == "" {
		t.Fatalf("target missing key: first=%+v second=%+v", targetA, targetB)
	}
	if targetA.Key != targetB.Key {
		t.Fatalf("keys differ for identical source: %q != %q", targetA.Key, targetB.Key)
	}
	if targetA.ID != targetB.ID {
		t.Fatalf("test fixture unexpectedly assigned different layout IDs: %q and %q", targetA.ID, targetB.ID)
	}
	condition := findNode(first.Body, "if / switch")
	if condition == nil || len(condition.Branches) != 1 || condition.Branches[0].Key == "" {
		t.Fatalf("branch missing key: %+v", condition)
	}
	if condition.Branches[0].Key != findNode(second.Body, "if / switch").Branches[0].Key {
		t.Fatal("branch key was not deterministic")
	}

	withInsertion := keyedFixture(append([]workflow.Operation{
		workflow.Set{Source: workflow.Source{File: "fixture.go", Line: 2}, Key: "inserted"},
	}, body...))
	insertedTarget := findNode(withInsertion.Body, "target")
	if insertedTarget.Key != targetA.Key {
		t.Fatalf("inserting another operation changed target key: %q != %q", insertedTarget.Key, targetA.Key)
	}
	if insertedTarget.ID == targetA.ID {
		t.Fatalf("insertion should demonstrate layout ID shift, both are %q", targetA.ID)
	}
}

func TestChangedOrDeletedPresentationSiteDoesNotReuseKey(t *testing.T) {
	original := keyedFixture([]workflow.Operation{
		workflow.Set{Source: workflow.Source{File: "fixture.go", Line: 10}, Key: "old"},
	})
	old := findNode(original.Body, "old")
	if old == nil || old.Key == "" {
		t.Fatalf("original key missing: %+v", old)
	}

	changed := keyedFixture([]workflow.Operation{
		workflow.Set{Source: workflow.Source{File: "fixture.go", Line: 10}, Key: "new"},
	})
	newNode := findNode(changed.Body, "new")
	if newNode == nil || newNode.Key == "" || newNode.Key == old.Key {
		t.Fatalf("changed operation reused key: old=%+v new=%+v", old, newNode)
	}
	if newNode.ID != old.ID {
		t.Fatalf("expected replacement to reuse sequential layout ID %q, got %q", old.ID, newNode.ID)
	}
	if findNode(changed.Body, "old") != nil {
		t.Fatal("removed operation remained in the presentation tree")
	}

	deleted := keyedFixture(nil)
	if findNode(deleted.Body, "old") != nil || findNode(deleted.Body, "new") != nil {
		t.Fatal("deleted operations remained in the presentation tree")
	}
}

func TestAmbiguousDuplicatePresentationFingerprintsAreUnkeyed(t *testing.T) {
	src := workflow.Source{File: "fixture.go", Line: 10}
	page := keyedFixture([]workflow.Operation{
		workflow.Set{Source: src, Key: "same"},
		workflow.Set{Source: src, Key: "same"},
	})
	if len(page.Body) != 2 {
		t.Fatalf("nodes=%d, want 2", len(page.Body))
	}
	if page.Body[0].ID == page.Body[1].ID {
		t.Fatalf("layout IDs should remain distinct: %q", page.Body[0].ID)
	}
	if page.Body[0].Key != "" || page.Body[1].Key != "" {
		t.Fatalf("ambiguous duplicates must not receive keys: %q %q", page.Body[0].Key, page.Body[1].Key)
	}
}
