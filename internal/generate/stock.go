package generate

import (
	"fmt"
	"go/format"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/tylergannon/gimbal/internal/builtin"
	"github.com/tylergannon/polytype/goload"
)

type stockField struct {
	Name, Type, JSON                      string
	Optional, RequiredString, RequiredInt bool
}

type stockData struct {
	Package, Entry, Name, Input, Params string
	Fields                              []stockField
	Optional                            bool
}

// Stock writes the command and invocation sources for the one compiled built-in
// selection. It reads each workflow's actual parameter type and graph roles.
func Stock(root string, session *goload.Session) error {
	_, err := new(StockGenerator).Generate(root, session, nil)
	return err
}

// StockGenerator retains extracted products for the fixed built-in selection.
// Callers select affected package names using their source/dependency tracking.
// A nil selection refreshes all products; an empty selection republishes cached
// products to repair outputs. A new instance always extracts missing products.
// It owns no retained goload.Session and must be called serially.
type StockGenerator struct {
	root     string
	products map[string]analysisResult
}

func (s *StockGenerator) Generate(root string, session *goload.Session, packages []string) (bool, error) {
	var published publication
	err := s.generate(root, session, packages, &published)
	return published.changed, err
}

func (s *StockGenerator) generate(root string, session *goload.Session, packages []string, published *publication) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if root != s.root || s.products == nil {
		s.root = root
		s.products = map[string]analysisResult{}
	}
	selectedPackages := map[string]bool{}
	for _, name := range packages {
		selectedPackages[name] = true
	}
	for _, selected := range builtin.Workflows {
		delete(selectedPackages, selected.Package)
	}
	if len(selectedPackages) != 0 {
		return fmt.Errorf("generate: unknown stock package selection: %v", selectedPackages)
	}
	for _, selected := range builtin.Workflows {
		_, cached := s.products[selected.Package]
		refresh := packages == nil
		for _, name := range packages {
			if name == selected.Package {
				refresh = true
			}
		}
		if cached && !refresh {
			continue
		}
		dir := filepath.Join(root, "internal", "workflows", selected.Package)
		// Each workflow has an independent persistent-cache slot. Editing one
		// workflow cannot invalidate extraction of every other stock workflow.
		results, err := analyze(root, map[string][]byte{filepath.Join(dir, "workflow_gen.go"): []byte("package " + selected.Package + "\n")}, []string{"./internal/workflows/" + selected.Package}, []analysisRequest{{dir, selected.Entry, selected.Name, true, true}}, session)
		if err != nil {
			return err
		}
		if err := check(results[0].Entry.entryInfo(), results[0].Graph); err != nil {
			return err
		}
		s.products[selected.Package] = results[0]
	}
	var pages []sourcePage
	var inputs []stockData
	if err := os.MkdirAll(filepath.Join(root, "internal", "invocation"), 0755); err != nil {
		return err
	}
	defaults := maps.Clone(builtin.Defaults())
	roleDefaults := map[string]string{}
	for k, v := range defaults {
		roleDefaults[string(k)] = v
	}
	for _, selected := range builtin.Workflows {
		result := s.products[selected.Package]
		graph, info := result.Graph, result.Entry.entryInfo()
		if err := check(info, graph); err != nil {
			return err
		}
		if err := published.remove(filepath.Join(root, "cmd", "gimbal", selected.Command)); err != nil {
			return err
		}
		data := stockData{Package: selected.Package, Entry: selected.Entry, Name: selected.Name, Input: "Start" + selected.Entry + "Input", Params: info.params}
		for _, field := range info.fields {
			if field.optional {
				data.Optional = true
			}
			data.Fields = append(data.Fields, stockField{Name: field.name, Type: field.kind, JSON: strings.ReplaceAll(field.flag, "-", "_"), Optional: field.optional, RequiredString: !field.optional && field.kind == "string", RequiredInt: !field.optional && field.kind == "int"})
		}
		p, err := pageGuide(result.Page, info, graph.Roles(), roleDefaults)
		if err != nil {
			return err
		}
		pages = append(pages, p)
		inputs = append(inputs, data)
		text := definitionSource("invocation", selected.Entry, info.params, info.fields, description(info, graph, p, roleDefaults), selected.Package+".Graph", selected.Package+".")
		text = strings.Replace(text, "\"github.com/tylergannon/gimbal\"", "\"github.com/tylergannon/gimbal\";\"github.com/tylergannon/gimbal/internal/workflows/"+selected.Package+"\"", 1)
		text += fmt.Sprintf("\nfunc register%s(r *gimbal.Registry) error {return r.RegisterWorkflow(%sDefinition())}\n", selected.Entry, selected.Entry)
		formatted, err := format.Source([]byte(text))
		if err != nil {
			return err
		}
		if err = published.write(filepath.Join(root, "internal", "invocation", selected.Package+"_gen.go"), formatted, 0644); err != nil {
			return err
		}

	}
	if err := writeStockRegistryResult(root, published); err != nil {
		return err
	}
	if err := writeWorkflowPagesResult(root, pages, inputs, published); err != nil {
		return err
	}
	if err := published.remove(filepath.Join(root, "cmd", "gimbal", "stock_workflows_gen.go")); err != nil {
		return err
	}
	return nil
}

func writeStockRegistry(root string) error {
	var p publication
	return writeStockRegistryResult(root, &p)
}
func writeStockRegistryResult(root string, published *publication) error {
	var registrations strings.Builder
	registrations.WriteString("//go:build !jsonschema\n\n// Code generated by internal/generate. DO NOT EDIT.\npackage invocation\nimport \"github.com/tylergannon/gimbal\"\nfunc Register(r *gimbal.Registry) error {\n")
	for _, selected := range builtin.Workflows {
		fmt.Fprintf(&registrations, "if err:=register%s(r);err!=nil{return err}\n", selected.Entry)
	}
	registrations.WriteString("return nil\n}\n")
	registration, err := format.Source([]byte(registrations.String()))
	if err != nil {
		return err
	}
	if err = published.write(filepath.Join(root, "internal", "invocation", "registry_gen.go"), registration, 0644); err != nil {
		return err
	}
	// Projection analysis excludes the executable workflow adapters. Keep the
	// public signature available without referencing those excluded functions.
	projection, err := format.Source([]byte(`//go:build jsonschema

// Code generated by internal/generate. DO NOT EDIT.
package invocation
import "github.com/tylergannon/gimbal"
func Register(*gimbal.Registry) error {
panic("invocation.Register is unavailable during jsonschema analysis")
}
`))
	if err != nil {
		return err
	}
	return published.write(filepath.Join(root, "internal", "invocation", "registry_projection_gen.go"), projection, 0644)
}
