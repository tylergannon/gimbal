// Package devgenerate is application-owned tooling for skgo dev. Production
// entrypoints must not import it: the helper process owns all retained workers.
package devgenerate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const module = "github.com/tylergannon/gimbal"

type producer struct {
	id, dir, entry, name string
	kind                 string
	outputs              []string
	dependencies         membership
}

func registry() []producer {
	all := []producer{
		{id: "schema:.", dir: ".", kind: "schema", outputs: []string{"jsonschema_gen.go", "jsonschema"}},
		{id: "workflow-types", dir: "workflow", kind: "workflow-types", outputs: []string{"workflow/polytype_gen.go", "web/src/lib/workflow/types.ts"}},
		{id: "observation-types", dir: "internal/observation", kind: "observation-types", outputs: []string{"web/src/lib/observation/types.ts"}},
		{id: "opencode", dir: "opencode", kind: "openapi", outputs: []string{"opencode/types.gen.go"}},
	}
	for _, pkg := range []string{"implementinterview", "implementation", "notebook", "pyramidsummary", "researcheval", "researchdocument", "review", "servicecheck"} {
		dir := "internal/workflows/" + pkg
		all = append(all, producer{id: "schema:" + dir, dir: dir, kind: "schema", outputs: []string{dir + "/jsonschema_gen.go", dir + "/polytype_gen.go", dir + "/jsonschema"}})
	}
	for _, item := range []struct{ pkg, entry, name string }{
		{"implementinterview", "ImplementInterview", "implement-interview"},
		{"implementation", "Implement", "implement"},
		{"conversation", "Conversation", "conversation"},
		{"pyramidsummary", "PyramidSummary", "pyramid-summary"},
		{"researcheval", "ResearchEval", "research-eval"},
		{"researchdocument", "ResearchDocument", "research-document"},
		{"review", "Review", "review"},
		{"servicecheck", "ServiceCheck", "service-check"},
		{"sprintplan", "SprintPlan", "sprint-plan"},
		{"validateproduct", "ValidateProduct", "validate-product"},
	} {
		dir := "internal/workflows/" + item.pkg
		all = append(all, producer{id: "graph:" + item.pkg, dir: dir, kind: "graph", entry: item.entry, name: item.name, outputs: []string{dir + "/workflow_gen.go"}})
	}
	return all
}

// Keep the registry explicit. A newly added or changed directive is a visible
// development error until its producer and ownership have been registered.
func validateDeclarations(root string, producers []producer) error {
	want := map[string]map[string]bool{}
	add := func(dir, line string) {
		if want[dir] == nil {
			want[dir] = map[string]bool{}
		}
		want[dir][line] = true
	}
	for _, p := range producers {
		switch p.kind {
		case "schema":
			add(p.dir, "go tool polytype --validate")
		case "workflow-types":
			add(p.dir, "go tool polytype --typescript ../web/src/lib/workflow")
		case "openapi":
			add(p.dir, "go tool oapi-codegen --config types.cfg.yaml types.json")
		case "graph":
			add(p.dir, "go run "+module+"/internal/generate/gimbalgen -entry "+p.entry+" -name "+p.name)
		}
	}
	// SKGO's own generation remains owned by skgo dev; the shipping appgen
	// command still provides the complete production invocation.
	add("internal/skgo", "go run ../generate/appgen generate --web ../../web --locals-package "+module+"/internal/app --hook-package "+module+"/internal/serverhooks --swift-out ../../native/Generated.swift --swift-remote src/lib/transcript.remote.ts#abortSession --swift-remote src/lib/transcript.remote.ts#acceptTranscript --swift-remote src/lib/transcript.remote.ts#activateSession --swift-remote src/lib/transcript.remote.ts#closeSession --swift-remote src/lib/transcript.remote.ts#openSession --swift-remote src/lib/transcript.remote.ts#touchSession --swift-remote src/lib/transcript.remote.ts#recordingHeader")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && excludedDirectory(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		dir, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		dir = filepath.ToSlash(dir)
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(line, "//go:generate ") {
				continue
			}
			command := strings.TrimSpace(strings.TrimPrefix(line, "//go:generate "))
			present, known := want[dir][command]
			if !known || !present {
				return fmt.Errorf("unregistered generation declaration in %s: %s; register the targeted development producer", path, command)
			}
			want[dir][command] = false
		}
		return nil
	})
	if err != nil {
		return err
	}
	for dir, commands := range want {
		for command, missing := range commands {
			if missing {
				return fmt.Errorf("registered generation declaration removed from %s: %s; reconcile its owned outputs before removing the registration", dir, command)
			}
		}
	}
	return nil
}

func excludedDirectory(name string) bool {
	return strings.HasPrefix(name, ".") || name == "ephemeral" || name == "third_party" || name == "node_modules" || name == "vendor" || name == "testdata"
}
