package devgenerate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Schema generation uses the pinned standalone source-first tool so all rules
// in schema.go stay authoritative. Snapshot only that producer's fixed owned
// outputs, including partial publication on command failure. Polytype itself
// suppresses equal writes and uses its persistent result caches.
func (g *Generator) schema(ctx context.Context, p producer) (bool, error) {
	before, err := snapshot(g.root, p.outputs)
	if err != nil {
		return false, err
	}
	cmd := exec.CommandContext(ctx, g.goCommand, "tool", "polytype", "--validate")
	cmd.Dir = filepath.Join(g.root, p.dir)
	cmd.Stdout, cmd.Stderr = diagnostics, diagnostics
	commandErr := cmd.Run()
	after, err := snapshot(g.root, p.outputs)
	return !maps.Equal(before, after), errors.Join(commandErr, err)
}

func snapshot(root string, outputs []string) (map[string][32]byte, error) {
	result := map[string][32]byte{}
	for _, output := range outputs {
		err := filepath.WalkDir(filepath.Join(root, output), func(path string, d fs.DirEntry, err error) error {
			if os.IsNotExist(err) {
				return nil
			}
			if err != nil {
				return err
			}
			if !d.IsDir() {
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				result[path] = sha256.Sum256(raw)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// The OpenAPI generator has one fixed application-owned output. Generate into
// a private file, then publish equal-aware bytes; the tool's own unconditional
// output writer must not create a false generation/restart on every callback.
func (g *Generator) openapi(ctx context.Context) (bool, error) {
	dir := filepath.Join(g.root, "opencode")
	raw, err := os.ReadFile(filepath.Join(dir, "types.cfg.yaml"))
	if err != nil {
		return false, err
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return false, err
	}
	if cfg["output"] != "types.gen.go" {
		return false, fmt.Errorf("OpenAPI output %q is not registered; expected types.gen.go", cfg["output"])
	}
	// Private tool output is not an application edit. Keeping scratch outside
	// the watched module lets a full generation pass acknowledge its raw epoch.
	scratch, err := os.MkdirTemp("", "gimbal-openapi-*")
	if err != nil {
		return false, err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	temporary := filepath.Join(scratch, "types.gen.go")
	// oapi-codegen's configured output overrides -o. Override only the fixed
	// output in a private configuration; otherwise the CLI writes the real
	// application file and leaves the private result empty.
	cfg["output"] = temporary
	configuration, err := yaml.Marshal(cfg)
	if err != nil {
		return false, err
	}
	configFile := filepath.Join(scratch, "types.cfg.yaml")
	if err := os.WriteFile(configFile, configuration, 0600); err != nil {
		return false, err
	}
	cmd := exec.CommandContext(ctx, g.goCommand, "tool", "oapi-codegen", "--config", configFile, "types.json")
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = diagnostics, diagnostics
	if err := cmd.Run(); err != nil {
		return false, err
	}
	content, err := os.ReadFile(temporary)
	if err != nil {
		return false, err
	}
	return publish(filepath.Join(dir, "types.gen.go"), content)
}

func publish(path string, content []byte) (bool, error) {
	old, err := os.ReadFile(path)
	if err == nil && bytes.Equal(old, content) {
		return false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		return false, err
	}
	return true, nil
}
