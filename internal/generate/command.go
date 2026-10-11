package generate

import (
	"fmt"

	"github.com/tylergannon/gimbal/workflow"
)

// check refuses a command whose flags would collide: a parameter field or a
// role named like a flag every command has, or like each other; and a roles
// entry for a role the workflow never creates a session for.
func check(info entryInfo, graph workflow.Graph) error {
	taken := map[string]string{"work-dir": "the Gimbal environment", "project": "the Gimbal environment", "instance-dir": "the Gimbal instance", "follow": "the Gimbal run", "help": "cobra", "input": "JSON input", "conversation": "conversation association", "task-endpoint": "task backend", "workflow-endpoint": "workflow backend", "workflow-project": "workflow environment", "task-route": "task backend route", "workflow-route": "workflow backend route"}
	jsonNames := map[string]bool{}
	for _, f := range info.fields {
		if jsonNames[f.json] {
			return fmt.Errorf("generate: duplicate JSON field %q", f.json)
		}
		jsonNames[f.json] = true
		if by, ok := taken[f.flag]; ok {
			return fmt.Errorf("generate: the parameter field %s would be --%s, which is %s's", f.name, f.flag, by)
		}
		taken[f.flag] = "a workflow parameter"
	}
	roles := graph.Roles()
	for _, role := range roles {
		if by, ok := taken[role]; ok {
			return fmt.Errorf("generate: the role %s would be --%s, which is %s's", role, role, by)
		}
	}
	return nil
}
