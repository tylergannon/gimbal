package sprintplan

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal"
)

func prepare(ctx context.Context, workdir string, params Params) (string, error) {
	if strings.TrimSpace(params.Intent) == "" || strings.TrimSpace(params.SprintDir) == "" {
		return "", fmt.Errorf("intent and sprint-dir are required")
	}
	intent, dir := params.Intent, params.SprintDir
	if !filepath.IsAbs(intent) {
		intent = filepath.Join(workdir, intent)
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workdir, dir)
	}
	code, _, stderr, err := gimbal.RunCommand(ctx, "prepare-sprint", workdir, "sh", "-c", `set -eu
 test -s "$1"
 mkdir -p -- "$(dirname "$2")"
 mkdir -- "$2"
 mkdir -p -- "$2/working-set/project" "$2/working-set/prior-art" "$2/draft" "$2/critique"
 cp -- "$1" "$2/intent.md"`, "prepare-sprint", intent, dir)
	if err != nil || code != 0 {
		return "", fmt.Errorf("prepare sprint: exit %d: %s: %w", code, stderr, err)
	}
	return dir, nil
}

func checkArtifact(ctx context.Context, path string, turnErr error) error {
	if turnErr != nil {
		return turnErr
	}
	code, body, stderr, err := gimbal.RunCommand(ctx, "read-artifact", filepath.Dir(path), "cat", "--", path)
	if err != nil || code != 0 {
		return fmt.Errorf("read artifact: exit %d: %s: %w", code, stderr, err)
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("empty artifact: %s", path)
	}
	return nil
}
