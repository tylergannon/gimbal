package notebook

import (
	"fmt"
	"strings"
)

// ValidateIndex checks the bounded fixture's reference integrity and route shape.
func ValidateIndex(index Index) error {
	if len(index.Routes) != 4 {
		return fmt.Errorf("want four retrieval routes, got %d", len(index.Routes))
	}
	seen := map[string]bool{}
	for _, route := range index.Routes {
		if strings.TrimSpace(route.Question) == "" || strings.TrimSpace(route.Answer) == "" || len(route.Sources) == 0 {
			return fmt.Errorf("empty retrieval route")
		}
		if seen[route.Question] {
			return fmt.Errorf("duplicate question %q", route.Question)
		}
		seen[route.Question] = true
		for _, source := range route.Sources {
			if source != "local" && source != "isolation" && source != "ownership" && source != "updates" {
				return fmt.Errorf("unknown source %q", source)
			}
		}
	}
	return nil
}

// ValidateSprint checks that all dependencies are defined earlier and checks exist.
func ValidateSprint(plan Sprint) error {
	if strings.TrimSpace(plan.Goal) == "" || len(plan.Tasks) < 3 || len(plan.Tasks) > 5 {
		return fmt.Errorf("want a goal and three to five tasks")
	}
	seen := map[string]bool{}
	for _, task := range plan.Tasks {
		if task.ID == "" || strings.TrimSpace(task.Outcome) == "" || strings.TrimSpace(task.Acceptance) == "" {
			return fmt.Errorf("task lacks outcome or acceptance")
		}
		if seen[task.ID] {
			return fmt.Errorf("duplicate task %q", task.ID)
		}
		for _, dependency := range task.DependsOn {
			if !seen[dependency] {
				return fmt.Errorf("dependency %q must precede %q", dependency, task.ID)
			}
		}
		seen[task.ID] = true
	}
	return nil
}
