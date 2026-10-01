package notebook

import "testing"

func TestIndexRejectsFabricatedSource(t *testing.T) {
	index := Index{Routes: []Route{{"one", []string{"local"}, "a"}, {"two", []string{"isolation"}, "b"}, {"three", []string{"ownership"}, "c"}, {"four", []string{"invented"}, "d"}}}
	if err := ValidateIndex(index); err == nil {
		t.Fatal("fabricated source accepted")
	}
	index.Routes[3].Sources = []string{"updates"}
	if err := ValidateIndex(index); err != nil {
		t.Fatal(err)
	}
}
func TestSprintRejectsForwardOrCyclicDependency(t *testing.T) {
	plan := Sprint{Goal: "trial", Tasks: []Task{{"baseline", "local", []string{"isolation"}, "run"}, {"isolation", "stacks", []string{"baseline"}, "check"}, {"cleanup", "stop", []string{"isolation"}, "observe"}}}
	if err := ValidateSprint(plan); err == nil {
		t.Fatal("cycle accepted")
	}
	plan.Tasks[0].DependsOn = nil
	if err := ValidateSprint(plan); err != nil {
		t.Fatal(err)
	}
}
