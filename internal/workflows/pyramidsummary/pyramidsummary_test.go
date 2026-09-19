package pyramidsummary

import (
	"fmt"
	"testing"

	"github.com/tylergannon/gimble/workflow"
)

func TestPyramidBudgets(t *testing.T) {
	tests := []struct {
		largest int
		want    string
	}{
		{3200, "[3200 1600 800 400 200 100]"},
		{10000, "[10000 5000 2500 1250 625 312 156]"},
		{199, "[199]"},
	}
	for _, test := range tests {
		if got := fmt.Sprint(tokenBudgets(test.largest)); got != test.want {
			t.Errorf("tokenBudgets(%d) = %s, want %s", test.largest, got, test.want)
		}
	}
}

func TestGeneratedGraphHasSixParallelCompressionsAndRepairs(t *testing.T) {
	if len(Graph.Diagnostics) != 0 {
		t.Fatalf("generated graph diagnostics = %+v", Graph.Diagnostics)
	}
	want := map[string]bool{"compressions": false, "repair": false}
	for _, operation := range Graph.Body {
		group, ok := operation.(workflow.Group)
		if !ok {
			continue
		}
		if _, tracked := want[group.Name]; !tracked {
			continue
		}
		want[group.Name] = true
		if len(group.Children) != 6 {
			t.Errorf("%s group has %d children, want 6", group.Name, len(group.Children))
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("generated graph has no %s group", name)
		}
	}
}

func TestIssuesForLevel(t *testing.T) {
	verdict := PyramidVerdict{Levels: []LevelVerdict{{Level: 2, MaterialIssues: []string{"missing core idea"}}, {Level: 4, MaterialIssues: []string{"too telegraphic"}}}}
	if got := issuesForLevel(verdict, 2); len(got) != 1 || got[0] != "missing core idea" {
		t.Fatalf("level 2 issues = %v", got)
	}
	if got := issuesForLevel(verdict, 3); len(got) != 0 {
		t.Fatalf("level 3 issues = %v, want none", got)
	}
}
