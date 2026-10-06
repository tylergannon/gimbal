// Package builtin holds the build-time selection and default role models for
// workflows shipped in the Gimbal binary.
package builtin

import (
	_ "embed"
	"encoding/json"

	"github.com/tylergannon/gimbal"
)

type Workflow struct {
	Package string
	Entry   string
	Name    string
	Command string
}

var Workflows = []Workflow{
	{Package: "sprintplan", Entry: "SprintPlan", Name: "sprint-plan", Command: "sprintplan_gen.go"},
	{Package: "review", Entry: "Review", Name: "review", Command: "review_gen.go"},
	{Package: "implementation", Entry: "Implement", Name: "implement", Command: "implementation_gen.go"},
	{Package: "researchdocument", Entry: "ResearchDocument", Name: "research-document", Command: "researchdocument_gen.go"},
	{Package: "researcheval", Entry: "ResearchEval", Name: "research-eval", Command: "researcheval_gen.go"},
	{Package: "pyramidsummary", Entry: "PyramidSummary", Name: "pyramid-summary", Command: "pyramidsummary_gen.go"},
	{Package: "validateproduct", Entry: "ValidateProduct", Name: "validate-product", Command: "validateproduct_gen.go"},
}

// defaults.json assigns models to the established roles in roles.go. It is not
// a place to invent workflow- or provider-specific roles. Existing noncatalog
// entries require migration; do not copy that pattern into new workflows.
// Track that work at https://github.com/tylergannon/gimbal-view/issues/14.
//
//go:embed defaults.json
var defaultsJSON []byte

func Defaults() map[gimbal.WorkflowRole]string {
	var defaults map[gimbal.WorkflowRole]string
	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil {
		panic(err)
	}
	return defaults
}
