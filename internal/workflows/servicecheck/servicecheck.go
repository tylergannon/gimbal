// Package servicecheck exercises an agent and a named HTTP service through runtime-loaded messaging.
// Both the agent and command fetch the response using producer-supplied bindings.
// Scope cleanup relinquishes use through the supplied orchestration dependency.
//
// Use a fresh, otherwise empty work directory for each concurrent run. Python 3
// must be available in the command environment. Task adds instructions to the
// agent. The operator supplies infrastructure; this workflow does not provision
// it. Each messaging producer must explicitly construct its service dependency.
// The default devops-tasks model is gpt-5.6-luna; --devops-tasks overrides it.
// Success means both consumers fetched HTTP 200 with GIMBAL_SERVICE_OK.
// Continuous availability and infrastructure provisioning are not validated.
//
// Example:
//
//	gimbal run service-check --work-dir /abs/empty --task-route @/abs/route.json --devops-tasks pi/diffusion/deepseek-4.1-flash --task "Report the fetched response"
package servicecheck

import (
	"context"
	"fmt"

	"github.com/tylergannon/gimbal"
)

//go:generate go tool polytype --validate
//go:generate go run github.com/tylergannon/gimbal/internal/generate/gimbalgen -entry ServiceCheck -name service-check

type Result struct {
	// Summary describes what the agent did.
	Summary string `json:"summary"`
}

type Params struct {
	// Task adds instructions after the agent fetches the supplied HTTP service.
	Task string
}

// ServiceCheck checks consumer-relative bindings and scoped service-use cleanup.
func ServiceCheck(ctx context.Context, env gimbal.Env, params Params) error {
	return gimbal.Scope(gimbal.WithServices(ctx, "http"), "service", func(ctx context.Context) error {
		gimbal.Set(ctx, "task", params.Task)
		agent := gimbal.NewSession(ctx, gimbal.RoleDevOpsTasks, env.WorkDir)
		if _, err := agent.Generate[Result](ctx, agentPrompt); err != nil {
			return err
		}
		code, _, _, err := gimbal.RunCommand(ctx, "consume", env.WorkDir, "python3", "-c", consume)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("service response check exited %d", code)
		}
		return nil
	})
}

const agentPrompt = "Fetch the runtime-supplied named http service URL from this agent's environment using an HTTP client. Require HTTP 200 and the response body GIMBAL_SERVICE_OK. Do not merely display the URL. Complete the additional task below, if any. Do not start or stop infrastructure. Return a short description of the actual response you fetched."
const consume = `import os, urllib.request
with urllib.request.urlopen(os.environ["GIMBAL_SERVICE_HTTP"], timeout=5) as response:
    assert response.status == 200, response.status
    body = response.read().decode().strip()
assert body == "GIMBAL_SERVICE_OK", body
print("Command fetched supplied service:", body)`
