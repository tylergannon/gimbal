package devgenerate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/builtin"
	"github.com/tylergannon/gimbal/internal/generate"
	"github.com/tylergannon/gimbal/workflow"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/polytype/codegen"
	"github.com/tylergannon/polytype/goload"
	"github.com/tylergannon/skgo/devgen"
)

// Generator retains only development tooling state, never an application or a
// project lock. Generate is called serially by the SKGO protocol adapter.
type Generator struct {
	root, goCommand       string
	producers             []producer
	initialized           bool
	observation, workflow codegen.Worker
	stock                 generate.StockGenerator
}

func New(root string) (*Generator, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Generator{root: root, goCommand: "go", producers: registry()}, nil
}

func (g *Generator) Close() error {
	return errors.Join(g.observation.Close(), g.workflow.Close())
}

func (g *Generator) Generate(ctx context.Context, request devgen.Request) (bool, error) {
	full := request.Full || !g.initialized
	paths := make([]string, len(request.Paths))
	for i, path := range request.Paths {
		if !filepath.IsAbs(path) {
			return false, fmt.Errorf("generation change path must be absolute: %s", path)
		}
		paths[i] = filepath.Clean(path)
	}
	selected, stockPackages, stockSelected, sourceChange := g.selectProducers(full, paths)
	changed := false
	if g.initialized && !full && g.stockDirectoryChange(paths) {
		// A directory rename can remove the invocation package itself. Repair
		// only known owned outputs from retained products before metadata
		// discovery, which may otherwise fail on that missing package. Later
		// selected source generation still runs in its ordinary order.
		var err error
		changed, err = g.stock.Generate(g.root, nil, []string{})
		if err != nil {
			return changed, fmt.Errorf("stock output repair: %w", err)
		}
	}
	finish := func(wrote bool, err error) (bool, error) { return changed || wrote, err }
	if full || sourceChange {
		if err := validateDeclarations(g.root, g.producers); err != nil {
			return changed, err
		}
		current, err := discover(ctx, g.root, g.goCommand, g.producers)
		if err != nil {
			// Retain previous memberships on failure. SKGO retains dirty paths
			// until the next successful pass, so deletion/repair cannot get lost.
			return changed, err
		}
		for i, p := range g.producers {
			for _, path := range paths {
				if !g.ownedOutput(path) && current[i].affected(path) {
					selected[i] = true
					if p.kind == "graph" {
						stockPackages = appendUnique(stockPackages, filepath.Base(p.dir))
						stockSelected = true
					}
				}
			}
		}
		// Publish membership only after successful generation below.
		return finish(g.run(ctx, selected, stockPackages, stockSelected, current))
	}
	return finish(g.run(ctx, selected, stockPackages, stockSelected, nil))
}

func (g *Generator) selectProducers(full bool, paths []string) ([]bool, []string, bool, bool) {
	selected := make([]bool, len(g.producers))
	stockPackages := []string{}
	stockSelected, sourceChange := full, false
	for i, p := range g.producers {
		selected[i] = full
		for _, path := range paths {
			if control(path) {
				selected[i], stockSelected, sourceChange = true, true, true
			}
			if strings.HasSuffix(path, ".go") && !g.ownedOutput(path) || isDirectoryEvent(path) {
				sourceChange = true
			}
			for _, output := range p.outputs {
				if within(path, filepath.Join(g.root, output)) || isDirectoryEvent(path) && within(filepath.Join(g.root, output), path) {
					selected[i] = true
				}
			}
			if !g.ownedOutput(path) && p.dependencies.affected(path) {
				selected[i] = true
			}
		}
		if selected[i] && p.kind == "graph" {
			stockPackages = appendUnique(stockPackages, filepath.Base(p.dir))
			stockSelected = true
		}
	}
	for _, path := range paths {
		for _, output := range g.stockOutputs() {
			if within(path, output) || isDirectoryEvent(path) && within(output, path) {
				stockSelected = true
			}
		}
		if within(path, filepath.Join(g.root, "internal/builtin")) || control(path) {
			stockSelected = true
			for _, p := range builtin.Workflows {
				stockPackages = appendUnique(stockPackages, p.Package)
			}
		}
	}
	if full {
		stockPackages = nil
	}
	return selected, stockPackages, stockSelected, sourceChange
}

func (g *Generator) ownedOutput(path string) bool {
	for _, output := range g.stockOutputs() {
		if within(path, output) {
			return true
		}
	}
	for _, p := range g.producers {
		for _, output := range p.outputs {
			if within(path, filepath.Join(g.root, output)) {
				return true
			}
		}
	}
	return false
}

func (g *Generator) stockOutputs() []string {
	outputs := []string{
		"internal/invocation/registry_gen.go", "internal/invocation/registry_projection_gen.go",
		"web/src/lib/workflow-design/generated/index.ts", "web/src/lib/workflow-design/generated/catalog.json",
		"cmd/gimbal/stock_workflows_gen.go",
	}
	for _, p := range builtin.Workflows {
		outputs = append(outputs, "internal/invocation/"+p.Package+"_gen.go", "cmd/gimbal/"+p.Command)
	}
	for i := range outputs {
		outputs[i] = filepath.Join(g.root, outputs[i])
	}
	return outputs
}

func (g *Generator) stockDirectoryChange(paths []string) bool {
	for _, path := range paths {
		if isDirectoryEvent(path) {
			for _, output := range g.stockOutputs() {
				if within(output, path) {
					return true
				}
			}
		}
	}
	return false
}

func (g *Generator) run(ctx context.Context, selected []bool, stockPackages []string, stockSelected bool, memberships []membership) (bool, error) {
	changed := false
	accept := func(wrote bool, err error) error {
		changed = changed || wrote
		return err
	}
	// Each invocation-scoped session is bounded by this call. Only independent
	// TypeScript Workers retain their own bounded sessions across callbacks.
	var session goload.Session
	for _, kind := range []string{"schema", "graph"} {
		for i, p := range g.producers {
			if !selected[i] || p.kind != kind {
				continue
			}
			if err := ctx.Err(); err != nil {
				return changed, err
			}
			var wrote bool
			var err error
			if kind == "schema" {
				wrote, err = g.schema(ctx, p)
			} else {
				wrote, err = generate.SourceResult(filepath.Join(g.root, p.dir), p.entry, p.name, "workflow_gen.go", "")
			}
			if err = accept(wrote, err); err != nil {
				return changed, fmt.Errorf("%s: %w", p.id, err)
			}
		}
	}
	if stockSelected {
		// Only built-in package names are accepted by StockGenerator. Authored
		// non-stock graphs (implementinterview) remain independent.
		var stock []string
		if stockPackages != nil {
			stock = []string{}
			for _, pkg := range stockPackages {
				for _, selected := range builtin.Workflows {
					if selected.Package == pkg {
						stock = appendUnique(stock, pkg)
					}
				}
			}
		}
		if err := accept(g.stock.Generate(g.root, &session, stock)); err != nil {
			return changed, fmt.Errorf("stock: %w", err)
		}
	}
	for i, p := range g.producers {
		if !selected[i] {
			continue
		}
		var wrote bool
		var err error
		switch p.kind {
		case "observation-types":
			wrote, err = g.observation.GenerateResult(polytype.Compose(
				polytype.DeclareType(polytype.TypeSpec{PackagePath: module + "/internal/observation", Name: "InteractiveRow"}),
				polytype.DeclareType(polytype.TypeSpec{PackagePath: module + "/internal/observation", Name: "TurnRow"}),
			), codegen.Options{TargetDir: filepath.Join(g.root, p.dir), TypeScript: &codegen.TypeScriptOptions{Dir: filepath.Join(g.root, "web/src/lib/observation")}})
		case "workflow-types":
			wrote, err = g.workflow.GenerateResult(polytype.Compose(
				polytype.DeclareType(polytype.TypeSpec{PackagePath: module + "/workflow", Name: "Graph"}),
				polytype.SealedUnion[workflow.Operation]("kind", polytype.Snake),
			), codegen.Options{TargetDir: filepath.Join(g.root, p.dir), TypeScript: &codegen.TypeScriptOptions{Dir: filepath.Join(g.root, "web/src/lib/workflow")}})
		case "openapi":
			wrote, err = g.openapi(ctx)
		default:
			continue
		}
		if err = accept(wrote, err); err != nil {
			return changed, fmt.Errorf("%s: %w", p.id, err)
		}
	}
	if memberships != nil {
		for i := range g.producers {
			g.producers[i].dependencies = memberships[i]
		}
	}
	g.initialized = true
	return changed, nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// Keep diagnostics off stdout, which belongs exclusively to devgen framing.
var diagnostics = os.Stderr
