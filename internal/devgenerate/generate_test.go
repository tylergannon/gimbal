package devgenerate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tylergannon/gimbal/internal/builtin"
	"github.com/tylergannon/skgo/devgen"
)

func TestStockDirectoryEventsSelectCachedRepair(t *testing.T) {
	g, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"internal/invocation", "web/src/lib/workflow-design", "web/src/lib/workflow-design/generated"} {
		t.Run(relative, func(t *testing.T) {
			path := filepath.Join(g.root, relative)
			_, packages, selected, _ := g.selectProducers(false, []string{path})
			if !selected || packages == nil || len(packages) != 0 || !g.stockDirectoryChange([]string{path}) {
				t.Fatalf("directory event selected=%v packages=%v", selected, packages)
			}
		})
	}
}

func TestCachedStockDirectoryRepairPrecedesDiscoveryFailure(t *testing.T) {
	// No authored workflow extraction is needed for this ordering fixture.
	// The generator package separately tests a fully populated stock cache.
	workflows := builtin.Workflows
	builtin.Workflows = nil
	t.Cleanup(func() { builtin.Workflows = workflows })
	g, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.stock.Generate(g.root, nil, nil); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(g.root, "internal/invocation")
	if err := os.RemoveAll(directory); err != nil {
		t.Fatal(err)
	}
	g.initialized = true
	g.producers = []producer{{id: "observation-types", dir: "internal/observation", kind: "observation-types"}}
	g.goCommand = "/does/not/exist"
	declaration, err := os.ReadFile("../skgo/config.go")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(g.root, "internal/skgo/config.go"), string(declaration), 0644)
	// An unrelated broken source/metadata selection must not suppress repair
	// from products already retained in the current helper.
	changed, err := g.Generate(context.Background(), devgen.Request{ID: 1, Paths: []string{directory}})
	if !changed || err == nil || !strings.Contains(err.Error(), "discover generation inputs") {
		t.Fatalf("repair before invalid fixture metadata changed=%v err=%v", changed, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "registry_gen.go")); err != nil {
		t.Fatalf("cached repair was blocked by source discovery: %v", err)
	}
}

func TestSelectionUsesDependenciesRemovedInputsAndOutputOwnership(t *testing.T) {
	root := t.TempDir()
	g, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(root, "shared")
	for i := range g.producers {
		p := &g.producers[i]
		p.dependencies = membership{dirs: map[string]bool{filepath.Join(root, p.dir): true}, files: map[string]bool{}}
		if p.id == "graph:review" || p.id == "observation-types" {
			p.dependencies.dirs[shared] = true
		}
		if p.id == "opencode" {
			p.dependencies.files[filepath.Join(root, "opencode/types.json")] = true
		}
	}
	for _, tc := range []struct {
		name, path string
		want       []string
	}{
		{"unrelated Svelte", "web/src/routes/+page.svelte", nil},
		{"deleted imported Go", "shared/gone.go", []string{"observation-types", "graph:review"}},
		{"deleted dependency directory", "shared", []string{"observation-types", "graph:review"}},
		{"missing observation output", "web/src/lib/observation/types.ts", []string{"observation-types"}},
		{"missing graph", "internal/workflows/interview/workflow_gen.go", []string{"graph:interview"}},
		{"missing non-Go input", "opencode/types.json", []string{"opencode"}},
		{"new Go member", "internal/workflows/review/new.go", []string{"graph:review", "schema:internal/workflows/review"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			selected, _, _, _ := g.selectProducers(false, []string{filepath.Join(root, tc.path)})
			got := map[string]bool{}
			for i, yes := range selected {
				if yes {
					got[g.producers[i].id] = true
				}
			}
			if len(got) != len(tc.want) {
				t.Fatalf("selected %v; want %v", got, tc.want)
			}
			for _, id := range tc.want {
				if !got[id] {
					t.Fatalf("missing %s from %v", id, got)
				}
			}
		})
	}
}

func TestRegisteredDeclarationsMatchApplication(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDeclarations(root, registry()); err != nil {
		t.Fatal(err)
	}
}

func TestDeclarationAdditionsAreVisible(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "new.go"), "package fixture\n//go:generate go tool imaginary\n", 0644)
	err := validateDeclarations(root, registry())
	if err == nil || !strings.Contains(err.Error(), "unregistered generation declaration") {
		t.Fatalf("wanted unsupported declaration error, got %v", err)
	}
}

func TestPartialSchemaWritesSurviveErrorAndNoOpRetry(t *testing.T) {
	root := t.TempDir()
	goCommand := filepath.Join(root, "go-fixture")
	write(t, goCommand, "#!/bin/sh\nprintf changed > jsonschema_gen.go\nexit 1\n", 0755)
	g := &Generator{root: root, goCommand: goCommand, producers: []producer{{id: "schema:.", dir: ".", kind: "schema", outputs: []string{"jsonschema_gen.go", "jsonschema"}}}}
	changed, err := g.run(context.Background(), []bool{true}, nil, false, nil)
	if !changed || err == nil || g.initialized {
		t.Fatalf("partial changed=%v error=%v initialized=%v", changed, err, g.initialized)
	}
	write(t, goCommand, "#!/bin/sh\nexit 0\n", 0755)
	changed, err = g.run(context.Background(), []bool{true}, nil, false, nil)
	if changed || err != nil || !g.initialized {
		t.Fatalf("retry changed=%v error=%v initialized=%v", changed, err, g.initialized)
	}
}

func TestOpenAPIPublicationSkipsEqualAndRepairsMissing(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "opencode")
	write(t, filepath.Join(dir, "types.cfg.yaml"), "output: types.gen.go\n", 0644)
	command := filepath.Join(root, "go-fixture")
	// The fixed adapter passes a temporary configuration. The fixture
	// intentionally writes unconditionally, as the actual OpenAPI CLI does.
	write(t, command, `#!/bin/sh
set -eu
test "$(basename "$PWD")" = opencode
application=$(cd .. && pwd)
while [ "$1" != '--config' ]; do shift; done
shift
case "$1" in "$application"/*) echo 'configuration scratch is watched' >&2; exit 1;; esac
output=$(sed -n 's/^output: //p' "$1")
case "$output" in "$application"/*) echo 'output scratch is watched' >&2; exit 1;; esac
printf 'package opencode\n' > "$output"
`, 0755)
	g := &Generator{root: root, goCommand: command}
	changed, err := g.openapi(context.Background())
	if !changed || err != nil {
		t.Fatalf("initial changed=%v err=%v", changed, err)
	}
	output := filepath.Join(dir, "types.gen.go")
	if raw, err := os.ReadFile(output); err != nil || string(raw) != "package opencode\n" {
		t.Fatalf("published OpenAPI bytes %q: %v", raw, err)
	}
	stamp := time.Unix(1234567890, 0)
	if err := os.Chtimes(output, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	changed, err = g.openapi(context.Background())
	info, statErr := os.Stat(output)
	if changed || err != nil || statErr != nil || !info.ModTime().Equal(stamp) {
		t.Fatalf("equal changed=%v err=%v statErr=%v", changed, err, statErr)
	}
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	changed, err = g.openapi(context.Background())
	if !changed || err != nil {
		t.Fatalf("repair changed=%v err=%v", changed, err)
	}
}

func TestUnrelatedRequestDoesNoWorkAfterInitialization(t *testing.T) {
	g, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	g.initialized = true
	g.goCommand = "/does/not/exist"
	changed, err := g.Generate(context.Background(), devgen.Request{ID: 1, Paths: []string{filepath.Join(g.root, "web/src/routes/+page.svelte")}})
	if changed || err != nil {
		t.Fatalf("unrelated changed=%v err=%v", changed, err)
	}
}

func write(t *testing.T, path, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}
