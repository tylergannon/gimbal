package devgenerate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Directories retain old Go membership as well as exact embedded files. A new
// or removed Go file and a removed directory can therefore invalidate a closure.
type membership struct {
	dirs  map[string]bool
	files map[string]bool
}

func (m membership) affected(path string) bool {
	if m.files[path] {
		return true
	}
	for dir := range m.dirs {
		if path == dir || strings.HasPrefix(dir, path+string(filepath.Separator)) || filepath.Dir(path) == dir && strings.HasSuffix(path, ".go") {
			return true
		}
	}
	return false
}

type listedPackage struct {
	ImportPath, Dir               string
	GoFiles, CgoFiles, EmbedFiles []string
	Imports                       []string
	Module                        *struct{ Dir string }
	Error                         *struct{ Err string }
}

// One metadata query covers every selected source root. jsonschema excludes
// stale generated graph/codec bodies, just as the ordinary producers do.
func discover(ctx context.Context, root, goCommand string, producers []producer) ([]membership, error) {
	args := []string{"list", "-e", "-deps", "-json", "-tags=jsonschema"}
	seen := map[string]bool{}
	for _, p := range producers {
		if p.kind != "openapi" && !seen[p.dir] {
			seen[p.dir] = true
			args = append(args, "./"+p.dir)
		}
	}
	cmd := exec.CommandContext(ctx, goCommand, args...)
	cmd.Dir = root
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("discover generation inputs: %w: %s", err, diagnostics.String())
	}
	packages := map[string]listedPackage{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var p listedPackage
		if err := decoder.Decode(&p); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if p.Error != nil {
			return nil, fmt.Errorf("discover generation inputs %s: %s", p.ImportPath, p.Error.Err)
		}
		packages[p.ImportPath] = p
	}
	result := make([]membership, len(producers))
	for i, p := range producers {
		m := membership{dirs: map[string]bool{}, files: map[string]bool{}}
		visited := map[string]bool{}
		var visit func(string)
		visit = func(name string) {
			if visited[name] {
				return
			}
			visited[name] = true
			pkg := packages[name]
			// Selected local modules are watched by SKGO. Immutable downloaded
			// modules need no per-event selection, but go.mod controls do.
			if pkg.Dir != "" && (pkg.Dir == root || strings.HasPrefix(pkg.Dir, root+string(filepath.Separator)) || pkg.Module != nil && !strings.Contains(filepath.ToSlash(pkg.Module.Dir), "/pkg/mod/")) {
				m.dirs[filepath.Clean(pkg.Dir)] = true
				for _, file := range append(append([]string{}, pkg.GoFiles...), append(pkg.CgoFiles, pkg.EmbedFiles...)...) {
					m.files[filepath.Join(pkg.Dir, file)] = true
				}
			}
			for _, dependency := range pkg.Imports {
				visit(dependency)
			}
		}
		if p.kind != "openapi" {
			name := module
			if p.dir != "." {
				name += "/" + p.dir
			}
			if _, ok := packages[name]; !ok {
				return nil, fmt.Errorf("generation source package missing: %s", name)
			}
			visit(name)
		} else {
			for _, file := range []string{"types.json", "types.cfg.yaml", "generate.go"} {
				m.files[filepath.Join(root, p.dir, file)] = true
			}
		}
		result[i] = m
	}
	return result, nil
}

func control(path string) bool {
	switch filepath.Base(path) {
	case "go.mod", "go.sum", "go.work", "go.work.sum":
		return true
	}
	return false
}

func within(path, directory string) bool {
	return path == directory || strings.HasPrefix(path, directory+string(filepath.Separator))
}

func isDirectoryEvent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir() || os.IsNotExist(err) && filepath.Ext(path) == ""
}
