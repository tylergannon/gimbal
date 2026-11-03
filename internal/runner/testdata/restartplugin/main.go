// Package restartplugin supplies the controlled native restart specimen.
package restartplugin

import (
	"context"
	"fmt"
	"strings"

	"github.com/tylergannon/gimbal"
)

type Params struct {
	RemoteDir string `json:"remote_dir"`
	Marker    string `json:"marker"`
	Token     string `json:"token"`
}

// Restart runs a native command and verifies its side effect after recovery.
func Restart(ctx context.Context, env gimbal.Env, p Params) error {
	ctx = gimbal.WithEnvironment(ctx, "task-remote")
	code, out, stderr, err := gimbal.RunCommand(ctx, "write-marker", p.RemoteDir, "sh", "-c", `printf '%s\n' "$2" >> "$1"; printf '%s\n%s\n%s\n' "$(hostname)" "$2" "$(wc -l < "$1" | tr -d ' ')"`, "sh", p.Marker, p.Token)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if code != 0 || len(lines) != 3 || lines[0] == "" || lines[1] != p.Token || lines[2] != "1" {
		return fmt.Errorf("write marker: code=%d output=%q stderr=%q", code, out, stderr)
	}
	code, out, stderr, err = gimbal.RunCommand(ctx, "verify-marker", p.RemoteDir, "sh", "-c", `test "$(wc -l < "$1" | tr -d ' ')" = 1 && test "$(cat "$1")" = "$2" && printf 'verified\n'`, "sh", p.Marker, p.Token)
	if err != nil {
		return err
	}
	if code != 0 || strings.TrimSpace(out) != "verified" {
		return fmt.Errorf("verify marker: code=%d output=%q stderr=%q", code, out, stderr)
	}
	code, _, stderr, err = gimbal.RunCommand(ctx, "remove-marker", p.RemoteDir, "rm", "--", p.Marker)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("remove marker: code=%d stderr=%q", code, stderr)
	}
	return nil
}
