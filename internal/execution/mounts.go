package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorktreeMounts returns only the project, selected worktree, run artifacts,
// Git metadata, and explicitly configured paths needed inside a worker. Git
// worktrees keep their administrative directory outside the checkout, so both
// it and its common object directory must also exist at their original paths.
func WorktreeMounts(project, workdir, artifacts string, configured ...string) ([]string, error) {
	project, err := filepath.Abs(project)
	if err != nil {
		return nil, fmt.Errorf("execution: resolve project: %w", err)
	}
	workdir, err = filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("execution: resolve workdir: %w", err)
	}
	artifacts, err = filepath.Abs(artifacts)
	if err != nil {
		return nil, fmt.Errorf("execution: resolve artifact directory: %w", err)
	}
	mounts := []string{filepath.Clean(project), filepath.Clean(workdir), filepath.Clean(artifacts)}
	for _, path := range configured {
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("execution: resolve configured mount: %w", err)
		}
		mounts = append(mounts, filepath.Clean(path))
	}
	worktree := workdir
	var gitEntry string
	for {
		candidate := filepath.Join(worktree, ".git")
		if _, statErr := os.Lstat(candidate); statErr == nil {
			gitEntry = candidate
			break
		} else if !os.IsNotExist(statErr) {
			return nil, fmt.Errorf("execution: inspect worktree Git metadata: %w", statErr)
		}
		parent := filepath.Dir(worktree)
		if parent == worktree {
			break
		}
		worktree = parent
	}
	if gitEntry == "" {
		return uniqueMounts(mounts), nil
	}
	mounts = append(mounts, worktree)
	info, err := os.Stat(gitEntry)
	if err != nil {
		return nil, fmt.Errorf("execution: inspect worktree Git metadata: %w", err)
	}
	if info.IsDir() {
		return uniqueMounts(mounts), nil
	}
	data, err := os.ReadFile(gitEntry)
	if err != nil {
		return nil, fmt.Errorf("execution: read worktree Git pointer: %w", err)
	}
	value := strings.TrimSpace(string(data))
	gitDirText, ok := strings.CutPrefix(value, "gitdir:")
	if !ok || strings.TrimSpace(gitDirText) == "" {
		return nil, fmt.Errorf("execution: invalid Git pointer in %s", gitEntry)
	}
	gitDir := strings.TrimSpace(gitDirText)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktree, gitDir)
	}
	gitDir, err = filepath.Abs(gitDir)
	if err != nil {
		return nil, fmt.Errorf("execution: resolve worktree Git directory: %w", err)
	}
	mounts = append(mounts, gitDir)
	commonFile := filepath.Join(gitDir, "commondir")
	if common, readErr := os.ReadFile(commonFile); readErr == nil {
		commonDir := strings.TrimSpace(string(common))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(gitDir, commonDir)
		}
		commonDir, err = filepath.Abs(commonDir)
		if err != nil {
			return nil, fmt.Errorf("execution: resolve Git common directory: %w", err)
		}
		mounts = append(mounts, commonDir)
	} else if !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("execution: read Git common directory: %w", readErr)
	}
	return uniqueMounts(mounts), nil
}

func uniqueMounts(mounts []string) []string {
	seen := make(map[string]bool, len(mounts))
	unique := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		mount = filepath.Clean(mount)
		if !seen[mount] {
			seen[mount] = true
			unique = append(unique, mount)
		}
	}
	return unique
}
