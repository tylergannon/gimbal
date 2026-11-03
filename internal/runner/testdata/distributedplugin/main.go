// Package main supplies a registered workflow for the opt-in distributed test.
package distributedplugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal"
	"github.com/tylergannon/gimbal/backend"
)

// Params supplies directories on their respective execution owners.
type Params struct {
	RemoteDir string `json:"remote_dir"`
}

// Mixed exercises a local command and a remote session through named messaging.
func Mixed(ctx context.Context, env gimbal.Env, params Params) error {
	ctx = gimbal.WithEnvironment(ctx, "workflow-local")
	code, output, stderr, err := gimbal.RunCommand(ctx, "local-command", env.WorkDir, "sh", "-c", `printf '%s\n' "$(hostname)" "$(uuidgen)" "$(uuidgen)"`)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("local command exited %d: %s", code, stderr)
	}
	parts := strings.Split(strings.TrimSpace(output), "\n")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return fmt.Errorf("invalid local command result: %q", output)
	}
	gimbal.Set(ctx, "local-host", parts[0])
	gimbal.Set(ctx, "nonce", parts[1])
	gimbal.Set(ctx, "shadow", "parent-value")
	marker := "buried-" + parts[2]
	gimbal.Set(ctx, "large-value", strings.Repeat("ordinary filler ", 10000)+marker+strings.Repeat(" ordinary filler", 10000))
	return gimbal.Scope(ctx, "remote", func(ctx context.Context) error {
		ctx = gimbal.WithEnvironment(ctx, "task-remote")
		gimbal.Set(ctx, "shadow", "child-value")
		agent := gimbal.NewSession(ctx, gimbal.RoleAgenticDialogues, params.RemoteDir)
		file := "file-" + parts[1]
		first, err := agent.Generate[gimbal.Text](gimbal.WithTaskFiles(ctx, backend.TaskFile{Name: "input.txt", Bytes: []byte(file)}), "Read the complete file for the large-value context, the supplied input.txt task file, and the visible nonce and shadow context values. Run hostname. Return exactly five lines: hostname, nonce, the token beginning buried-, the complete task file content, and shadow. No explanation.")
		if err != nil {
			return err
		}
		lines := strings.Split(strings.TrimSpace(string(first)), "\n")
		if len(lines) != 5 || lines[0] == "" || lines[0] == parts[0] || lines[1] != parts[1] || lines[2] != marker || lines[3] != file || lines[4] != "child-value" {
			return fmt.Errorf("remote result mismatch: %q", first)
		}
		second, err := agent.Generate[gimbal.Text](ctx, "From your previous answer, return exactly the fourth line, without explanation.")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(second)) != file {
			return fmt.Errorf("follow-up mismatch: %q", second)
		}
		return nil
	})
}
