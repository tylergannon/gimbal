package resources

import (
	"os"
	"path/filepath"
	"strings"
)

// contextFileCandidates are the per-directory instruction file names, in
// precedence order.
var contextFileCandidates = []string{"AGENTS.override.md", "AGENTS.md", "AGENTS.MD", "CLAUDE.md", "CLAUDE.MD"}

// loadContextFileFromDir returns the first readable instruction file in dir.
func loadContextFileFromDir(dir string) (ContextFile, bool) {
	for _, name := range contextFileCandidates {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return ContextFile{Path: path, Content: stripBOM(string(data))}, true
	}
	return ContextFile{}, false
}

// gitPaths carries the git metadata locations the context-file shadow check
// consumes.
type gitPaths struct {
	repoDir      string
	commonGitDir string
}

// findGitPaths walks up from cwd for a .git entry, handling a regular repo and
// a linked worktree whose .git file holds a gitdir pointer.
func findGitPaths(cwd string) (gitPaths, bool) {
	dir := cwd
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		switch {
		case err != nil:
			// Keep climbing.
		case info.Mode().IsRegular():
			content, readErr := os.ReadFile(gitPath)
			if readErr != nil {
				return gitPaths{}, false
			}
			rest, ok := strings.CutPrefix(strings.TrimSpace(string(content)), "gitdir: ")
			if ok {
				gitDir := resolveFrom(dir, strings.TrimSpace(rest))
				if !fileExists(filepath.Join(gitDir, "HEAD")) {
					return gitPaths{}, false
				}
				commonGitDir := gitDir
				if data, cerr := os.ReadFile(filepath.Join(gitDir, "commondir")); cerr == nil {
					commonGitDir = resolveFrom(gitDir, strings.TrimSpace(string(data)))
				}
				return gitPaths{repoDir: dir, commonGitDir: commonGitDir}, true
			}
		case info.IsDir():
			if !fileExists(filepath.Join(gitPath, "HEAD")) {
				return gitPaths{}, false
			}
			return gitPaths{repoDir: dir, commonGitDir: gitPath}, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return gitPaths{}, false
		}
		dir = parent
	}
}

// resolveFrom is Node's path.resolve for a single segment.
func resolveFrom(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, p)
}

// findShadowedContextFile returns the main repo's context file that a nested
// linked worktree's own copy shadows.
func findShadowedContextFile(cwd string) (string, bool) {
	paths, ok := findGitPaths(cwd)
	if !ok {
		return "", false
	}
	commonGitDir := canonicalizePath(paths.commonGitDir)
	worktreeRoot := canonicalizePath(paths.repoDir)
	mainRepoRoot := filepath.Dir(commonGitDir)
	if !strings.HasPrefix(worktreeRoot, mainRepoRoot+string(filepath.Separator)) {
		return "", false
	}
	if canonicalizePath(filepath.Join(mainRepoRoot, ".git")) != commonGitDir {
		return "", false
	}
	contextFile, ok := loadContextFileFromDir(worktreeRoot)
	if !ok {
		return "", false
	}
	return filepath.Join(mainRepoRoot, filepath.Base(contextFile.Path)), true
}

// LoadProjectContextFiles discovers context files: the global one under
// agentDir first, then each ancestor directory of cwd from root down to cwd.
func LoadProjectContextFiles(cwd, agentDir string) []ContextFile {
	resolvedCwd := resolvePath(cwd, ".")
	resolvedAgentDir := resolvePath(agentDir, ".")

	files := []ContextFile{}
	seen := map[string]bool{}

	if globalContext, ok := loadContextFileFromDir(resolvedAgentDir); ok && resolvedAgentDir != "" {
		files = append(files, globalContext)
		seen[globalContext.Path] = true
	}

	shadowed, hasShadowed := findShadowedContextFile(resolvedCwd)

	ancestors := []ContextFile{}
	currentDir := resolvedCwd
	for {
		if contextFile, ok := loadContextFileFromDir(currentDir); ok &&
			(!hasShadowed || canonicalizePath(contextFile.Path) != shadowed) &&
			!seen[contextFile.Path] {
			ancestors = append([]ContextFile{contextFile}, ancestors...)
			seen[contextFile.Path] = true
		}
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}
		currentDir = parentDir
	}
	files = append(files, ancestors...)
	return files
}
