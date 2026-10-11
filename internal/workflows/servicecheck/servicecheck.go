// Package servicecheck exercises an agent and a foreground HTTP service in one
// scope. The agent writes agent-result.txt; a command reads that file through
// the service. A final command verifies that scope exit stopped the service.
//
// Use a fresh, otherwise empty work directory for each concurrent run. Python 3
// and zsh must be available in the selected task environment. The workflow
// creates service-address and agent-result.txt there. Task adds instructions to
// the agent; it can ask for a longer operation when observing live controls.
// The default devops-tasks model is gpt-5.6-luna; --devops-tasks overrides it.
// Success means the HTTP response matched the agent's file and the service was
// unreachable after scope exit, not that any wider application was validated.
//
// Example:
//
//	gimbal run service-check --work-dir /abs/empty --task-endpoint http://127.0.0.1:8081 --devops-tasks pi/diffusion/deepseek-4.1-flash --task "Write the file now"
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
	// Task adds instructions for the agent before it writes agent-result.txt.
	Task string
}

// ServiceCheck checks shared agent/command work and automatic service cleanup.
func ServiceCheck(ctx context.Context, env gimbal.Env, params Params) error {
	err := gimbal.Scope(ctx, "service", func(ctx context.Context) error {
		if err := gimbal.Service(ctx, "http", env.WorkDir, serve); err != nil {
			return err
		}
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
	if err != nil {
		return err
	}
	code, _, _, err := gimbal.RunCommand(ctx, "released", env.WorkDir, "python3", "-c", released)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("service remained reachable after scope exit")
	}
	return nil
}

const agentPrompt = "The scope owns a foreground HTTP service. Wait until service-address exists. Complete the additional task below, if any. Then write the single line GIMBAL_SERVICE_OK to agent-result.txt in your working directory. Do not start or stop the service. Return a short description of what you did."
const serve = `exec python3 -u -c 'import http.server, pathlib
server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), http.server.SimpleHTTPRequestHandler)
pathlib.Path("service-address").write_text("http://127.0.0.1:" + str(server.server_port))
print("Service ready", server.server_port, flush=True)
server.serve_forever()'`
const consume = `import pathlib, urllib.request
address = pathlib.Path("service-address").read_text()
with urllib.request.urlopen(address + "/agent-result.txt", timeout=5) as response:
    body = response.read().decode().strip()
assert body == "GIMBAL_SERVICE_OK", body
print("Service returned the agent's file:", body)`
const released = `import pathlib, urllib.request, urllib.error
address = pathlib.Path("service-address").read_text()
try:
    urllib.request.urlopen(address, timeout=2)
except urllib.error.URLError:
    print("Scope released its service; parent command remains usable")
else:
    raise RuntimeError("service is still running")`
