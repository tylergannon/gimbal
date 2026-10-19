// Package invocation owns stock workflow input and dispatch, independent of the web.
package invocation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/internal/taskdaemon"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/effects"
)

type Request struct {
	WorkflowProject string                         `json:"workflow_project,omitempty"`
	RunID           string                         `json:"run_id,omitempty"`
	WorkflowRoute   backend.Route                  `json:"workflow_route,omitzero"`
	TaskRoute       backend.Route                  `json:"task_route,omitzero"`
	PluginDirs      []string                       `json:"plugin_dirs,omitempty"`
	TaskEndpoint    string                         `json:"task_endpoint,omitempty"`
	Workflow        string                         `json:"workflow"`
	Input           json.RawMessage                `json:"input"`
	Project         string                         `json:"project"`
	WorkDir         string                         `json:"work_dir"`
	Conversation    string                         `json:"conversation,omitempty"`
	Roles           map[gimbal.WorkflowRole]string `json:"roles,omitempty"`
}
type Accepted struct {
	RunID string `json:"run_id"`
}

func init() {
	if err := registerResourceBackends(gimbal.DefaultRegistry); err != nil {
		panic(err)
	}
	if err := gimbal.DefaultRegistry.RegisterBackend(gimbal.BackendDefinition{ExecutionIdentity: true, Name: "local", ConfigurationSchema: json.RawMessage(`{"type":"object","additionalProperties":false}`), ValidateConfiguration: func(raw json.RawMessage) error {
		if len(raw) == 0 {
			return nil
		}
		var value map[string]json.RawMessage
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		if value == nil || len(value) > 0 {
			return fmt.Errorf("local backend configuration must be an empty object")
		}
		return nil
	}, Connect: func(context.Context, json.RawMessage) (backend.Connection, error) {
		daemon := taskdaemon.New("")
		daemon.Secrets = backend.NewMemorySecrets()
		return daemon.Connection(), nil
	}}); err != nil {
		panic(err)
	}
	if err := Register(gimbal.DefaultRegistry); err != nil {
		panic(err)
	}
}
func LoadPlugins(dirs []string) error { return gimbal.DefaultRegistry.LoadPlugins(dirs) }

func Prepare(r Request) (map[gimbal.WorkflowRole]gimbal.ModelBinding, func(context.Context) error, error) {
	return PrepareContext(context.Background(), r)
}
func PrepareContext(ctx context.Context, r Request) (map[gimbal.WorkflowRole]gimbal.ModelBinding, func(context.Context) error, error) {
	definition, ok := gimbal.DefaultRegistry.Workflow(r.Workflow)
	if !ok {
		return nil, nil, fmt.Errorf("unknown workflow %q", r.Workflow)
	}
	prepared, err := definition.Prepare(r.Input)
	body := func(ctx context.Context) error { return prepared(ctx, gimbal.Env{WorkDir: r.WorkDir}) }
	if err != nil {
		return nil, nil, err
	}
	graph, _ := gimbal.RegisteredGraph(r.Workflow)
	specs := map[gimbal.WorkflowRole]string{}
	defaults := definition.Roles
	for _, name := range graph.Roles() {
		role := gimbal.WorkflowRole(name)
		specs[role] = defaults[role]
	}
	for role, spec := range r.Roles {
		if _, ok := specs[role]; !ok {
			return nil, nil, fmt.Errorf("unknown role %q for %s", role, r.Workflow)
		}
		specs[role] = spec
	}
	models, err := binding.Roles(specs)
	connection := effects.Connection(ctx)

	if err == nil && (r.TaskEndpoint != "" || (connection.Execution != nil || connection.Messages != nil)) {
		for role, spec := range specs {
			name, effort, hasEffort := strings.Cut(spec, ":")
			resolved, e := modelalias.Resolve(modelalias.Selection{Name: name, Effort: effort, EffortPresent: hasEffort})
			if e != nil {
				return nil, nil, e
			}
			m := models[role]
			m.Adapter = &taskdaemon.Harness{Endpoint: r.TaskEndpoint, Name: resolved.Harness, Connection: connection}
			models[role] = m
		}
	}
	return models, body, err
}
