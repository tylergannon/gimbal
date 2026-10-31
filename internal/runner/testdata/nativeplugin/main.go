package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/workflow"
)

func Register(registry *gimbal.Registry) error {
	return registry.RegisterWorkflow(gimbal.WorkflowDefinition{
		WorkflowDescription: gimbal.WorkflowDescription{
			Name: "native-admission-test", InputSchema: json.RawMessage(`{"type":"object"}`),
		},
		Graph: workflow.Graph{Name: "native-admission-test"},
		Prepare: func(json.RawMessage) (gimbal.Invocation, error) {
			return func(_ context.Context, env gimbal.Env) error {
				return os.WriteFile(filepath.Join(env.WorkDir, "executed"), []byte("native workflow ran"), 0600)
			}, nil
		},
	})
}
