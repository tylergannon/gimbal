// Package invocation owns stock workflow input and dispatch, independent of the web.
package invocation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tylergannon/gimbal/internal/modelalias"
	"github.com/tylergannon/gimbal/internal/taskdaemon"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/binding"
	"github.com/tylergannon/gimbal/internal/builtin"
)

type Request struct {
	TaskEndpoint string                         `json:"task_endpoint,omitempty"`
	Workflow     string                         `json:"workflow"`
	Input        json.RawMessage                `json:"input"`
	Project      string                         `json:"project"`
	WorkDir      string                         `json:"work_dir"`
	Conversation string                         `json:"conversation,omitempty"`
	Roles        map[gimbal.WorkflowRole]string `json:"roles,omitempty"`
}
type Accepted struct {
	RunID string `json:"run_id"`
}

var adapters = map[string]func(json.RawMessage, gimbal.Env) (func(context.Context) error, error){}

func Prepare(r Request) (map[gimbal.WorkflowRole]gimbal.ModelBinding, func(context.Context) error, error) {
	adapter := adapters[r.Workflow]
	if adapter == nil {
		return nil, nil, fmt.Errorf("unknown workflow %q", r.Workflow)
	}
	body, err := adapter(r.Input, gimbal.Env{WorkDir: r.WorkDir})
	if err != nil {
		return nil, nil, err
	}
	graph, _ := gimbal.RegisteredGraph(r.Workflow)
	specs := map[gimbal.WorkflowRole]string{}
	defaults := builtin.Defaults()
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
	if err == nil && r.TaskEndpoint != "" {
		for role, spec := range specs {
			name, effort, hasEffort := strings.Cut(spec, ":")
			resolved, e := modelalias.Resolve(modelalias.Selection{Name: name, Effort: effort, EffortPresent: hasEffort})
			if e != nil {
				return nil, nil, e
			}
			m := models[role]
			m.Adapter = &taskdaemon.Harness{Endpoint: r.TaskEndpoint, Name: resolved.Harness}
			models[role] = m
		}
	}
	return models, body, err
}
func decode(raw json.RawMessage, value any) error {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return fmt.Errorf("input must be a JSON object")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("input must contain one JSON object")
	}
	return nil
}
