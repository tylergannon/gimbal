package builtin

import (
	"embed"
	"fmt"
)

// Workflow pages are generated from the shipped source. Installed Gimbal never
// loads a checkout or starts a compiler to display them.
//
//go:embed pages/*.json
var workflowPages embed.FS

func WorkflowPageJSON(name string) (string, error) {
	for _, w := range Workflows {
		if w.Name == name {
			data, err := workflowPages.ReadFile("pages/" + name + ".json")
			return string(data), err
		}
	}
	return "", fmt.Errorf("unknown workflow %q", name)
}
