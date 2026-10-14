package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// The supervisor is compiled once. If one of its local implementation inputs
// changes, it must ask for a restart instead of running the old generator code.
func generatorFiles(root string) ([]string, error) {
	cmd := exec.Command("go", "list", "-deps", "-json", "./internal/generate/appgen")
	cmd.Dir = root
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	files := map[string]bool{}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var pkg struct {
			Dir                       string
			EmbedFiles, EmbedPatterns []string
			Module                    *struct {
				Main    bool
				GoMod   string
				Replace *struct{ Version, GoMod string }
			}
		}
		if err := decoder.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}
		if pkg.Module == nil || (!pkg.Module.Main && (pkg.Module.Replace == nil || pkg.Module.Replace.Version != "")) {
			continue
		}
		files[filepath.Join(pkg.Dir, "*.go")] = true
		for _, name := range pkg.EmbedFiles {
			files[filepath.Join(pkg.Dir, name)] = true
		}
		for _, pattern := range pkg.EmbedPatterns {
			files[filepath.Join(pkg.Dir, strings.TrimPrefix(pattern, "all:"))] = true
		}
		mod := pkg.Module.GoMod
		if pkg.Module.Replace != nil {
			mod = pkg.Module.Replace.GoMod
		}
		if mod != "" {
			files[mod] = true
			files[filepath.Join(filepath.Dir(mod), "go.sum")] = true
		}
	}
	cmd = exec.Command("go", "env", "GOWORK")
	cmd.Dir = root
	if raw, err := cmd.Output(); err != nil {
		return nil, err
	} else if work := strings.TrimSpace(string(raw)); work != "" && work != "off" {
		files[work] = true
		files[work+".sum"] = true
	}
	var result []string
	for file := range files {
		result = append(result, file)
	}
	slices.Sort(result)
	return result, nil
}
