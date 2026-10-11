package routes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/internal/app"
	"github.com/tylergannon/gimbal/internal/host"
	"github.com/tylergannon/gimbal/internal/invocation"
	"github.com/tylergannon/gimbal/internal/skgo/params"
	"github.com/tylergannon/skgo"
)

type LaunchField struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Include bool   `json:"include"`
}
type LaunchRole struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type LaunchWorkflow struct {
	Workflow string        `json:"workflow"`
	Project  string        `json:"project"`
	Workdir  string        `json:"workdir"`
	Inputs   []LaunchField `json:"inputs"`
	Roles    []LaunchRole  `json:"roles"`
}
type LaunchAccepted struct {
	RunID     string `json:"run_id"`
	ProjectID string `json:"project_id"`
}

func launchInput(d gimbal.WorkflowDescription, fields []LaunchField) (json.RawMessage, error) {
	values := map[string]json.RawMessage{}
	indexes := map[string]int{}
	for i, field := range fields {
		path := fmt.Sprintf("inputs[%d].value", i)
		if _, ok := indexes[field.Name]; ok {
			return nil, skgo.Invalidf(path, "Duplicate input %s", field.Name)
		}
		indexes[field.Name] = i
		index := slices.IndexFunc(d.Fields, func(f gimbal.WorkflowField) bool { return f.Name == field.Name })
		if index < 0 {
			return nil, skgo.Invalidf(path, "Unknown input %s", field.Name)
		}
		declared := d.Fields[index]
		if declared.Optional && !field.Include {
			continue
		}
		if declared.Nullable && field.Value == "null" {
			values[field.Name] = json.RawMessage("null")
			continue
		}
		switch declared.Kind {
		case "string":
			values[field.Name], _ = json.Marshal(field.Value)
		case "int":
			n, err := strconv.Atoi(field.Value)
			if err != nil {
				return nil, skgo.Invalidf(path, "Give an integer")
			}
			values[field.Name], _ = json.Marshal(n)
		case "bool":
			if field.Value != "true" && field.Value != "false" {
				return nil, skgo.Invalidf(path, "Choose true or false")
			}
			values[field.Name] = json.RawMessage(field.Value)
		}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	if err = gimbal.ValidateWorkflowInput(d, raw); err != nil {
		return nil, launchInputError(err, indexes)
	}
	return raw, nil
}
func launchInputError(err error, indexes map[string]int) error {
	var input *gimbal.InputError
	if errors.As(err, &input) {
		if i, ok := indexes[input.Field]; ok {
			return skgo.Invalidf(fmt.Sprintf("inputs[%d].value", i), "%s", input.Message)
		}
		return skgo.Invalidf("inputs", "%s", input.Error())
	}
	return err
}
func launchWorkflow(ctx context.Context, event app.RequestEvent[params.Params], arg LaunchWorkflow) (LaunchAccepted, error) {
	definition, ok := gimbal.DefaultRegistry.Workflow(arg.Workflow)
	if !ok {
		return LaunchAccepted{}, skgo.Invalidf("workflow", "Unknown workflow")
	}
	input, err := launchInput(definition.WorkflowDescription, arg.Inputs)
	if err != nil {
		return LaunchAccepted{}, err
	}
	indexes := map[string]int{}
	for i, f := range arg.Inputs {
		indexes[f.Name] = i
	}
	if _, err := definition.Prepare(input); err != nil {
		return LaunchAccepted{}, launchInputError(err, indexes)
	}
	roles := map[gimbal.WorkflowRole]string{}
	for i, role := range arg.Roles {
		key := gimbal.WorkflowRole(role.Name)
		if _, ok := definition.Roles[key]; !ok {
			return LaunchAccepted{}, skgo.Invalidf(fmt.Sprintf("roles[%d].value", i), "Unknown role")
		}
		if _, ok := roles[key]; ok {
			return LaunchAccepted{}, skgo.Invalidf(fmt.Sprintf("roles[%d].value", i), "Duplicate role")
		}
		if strings.TrimSpace(role.Value) != "" {
			roles[key] = role.Value
		}
	}
	owner := host.OwnerFrom(ctx)
	if owner == nil {
		return LaunchAccepted{}, skgo.Errorf(http.StatusServiceUnavailable, "Workflow host unavailable")
	}
	project, err := owner.Project(arg.Project)
	if err != nil {
		return LaunchAccepted{}, skgo.Invalidf("project", "Choose an admitted project")
	}
	id, err := project.Start(invocation.Request{Workflow: arg.Workflow, Input: input, WorkDir: arg.Workdir, Roles: roles})
	if err != nil {
		return LaunchAccepted{}, err
	}
	return LaunchAccepted{RunID: id, ProjectID: project.ID()}, nil
}

var _ = skgo.Form(launchWorkflow)
