package workflow

import (
	"slices"
	"testing"
)

func TestRolesAreEveryNewSessionOnceInSourceOrder(t *testing.T) {
	g := Graph{Body: []Operation{
		Session{Name: "researcher"},
		Session{Name: "planner", From: "researcher"},
		Group{Children: []GroupChild{
			{Body: []Operation{Session{Name: "claude"}}},
			{Body: []Operation{Session{Name: "claude"}}},
		}},
		Loop{Body: []Operation{
			Scope{Body: []Operation{Session{Name: "supervisor"}}},
			Condition{Branches: []Branch{{Body: []Operation{Session{Name: "judge"}}}}},
		}},
	}}
	if got := g.Roles(); !slices.Equal(got, []string{"researcher", "claude", "supervisor", "judge"}) {
		t.Fatalf("Roles = %q", got)
	}
}
