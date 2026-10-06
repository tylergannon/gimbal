package sprintplan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// prepare keeps filesystem setup out of the workflow's sequence of agent work.
func prepare(workdir string, params Params) (string, error) {
	if strings.TrimSpace(params.Intent) == "" || strings.TrimSpace(params.SprintDir) == "" {
		return "", fmt.Errorf("intent and sprint-dir are required")
	}
	intent := params.Intent
	if !filepath.IsAbs(intent) {
		intent = filepath.Join(workdir, intent)
	}
	body, err := os.ReadFile(intent)
	if err != nil {
		return "", fmt.Errorf("read intent: %w", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return "", fmt.Errorf("intent is empty")
	}
	dir := params.SprintDir
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(workdir, dir)
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", fmt.Errorf("create new sprint directory: %w", err)
	}
	for _, child := range []string{"working-set/project", "working-set/prior-art", "draft", "critique"} {
		if err := os.MkdirAll(filepath.Join(dir, child), 0o755); err != nil {
			return "", err
		}
	}
	return dir, os.WriteFile(filepath.Join(dir, "intent.md"), body, 0o644)
}

// checkArtifact checks the file the agent authored, never its status message.
func checkArtifact(path string, turnErr error) error {
	if turnErr != nil {
		return turnErr
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("empty artifact: %s", path)
	}
	return nil
}
