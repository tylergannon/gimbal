// appgen composes Gimbal's stock source, SKGO bindings and observation types.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/generate"
	"github.com/tylergannon/polytype/grammar"
	"github.com/tylergannon/polytype/typescript"
	skgo "github.com/tylergannon/skgo/generate"
	"golang.org/x/tools/go/packages"
)

type remotes []string

func (r *remotes) String() string         { return fmt.Sprint([]string(*r)) }
func (r *remotes) Set(value string) error { *r = append(*r, value); return nil }

func main() {
	if len(os.Args) < 2 || os.Args[1] != "generate" {
		log.Fatal("usage: appgen generate [options]")
	}
	flags := flag.NewFlagSet("generate", flag.ExitOnError)
	var cfg skgo.Config
	var selected remotes
	flags.StringVar(&cfg.Web, "web", "", "Vite root")
	flags.StringVar(&cfg.LocalsPackage, "locals-package", "", "locals package")
	flags.StringVar(&cfg.HookPackage, "hook-package", "", "hook package")
	flags.StringVar(&cfg.SwiftOut, "swift-out", "", "generated Swift file")
	flags.Var(&selected, "swift-remote", "remote selected for Swift")
	_ = flags.Parse(os.Args[2:])
	cfg.SwiftRemotes = selected
	var err error
	cfg.Out, err = os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	root, err := filepath.Abs(filepath.Join(cfg.Out, "../.."))
	if err != nil {
		log.Fatal(err)
	}
	if err = run(root, cfg); err != nil {
		log.Fatal(err)
	}
}

func run(root string, cfg skgo.Config) error {
	// Stock changes authored route inputs, so publish it before taking a snapshot.
	if err := generate.Stock(root); err != nil {
		return err
	}
	load := &packages.Config{Dir: root, Mode: grammar.LoadMode | packages.NeedDeps, BuildFlags: []string{"-tags=jsonschema"}}
	roots, err := packages.Load(load,
		"github.com/tylergannon/gimbal/internal/observation",
		"github.com/tylergannon/gimbal/internal/projectfiles",
		"github.com/tylergannon/gimbal/internal/transcript",
	)
	if err != nil {
		return err
	}
	var observation *packages.Package
	for _, pkg := range roots {
		if pkg.PkgPath == "github.com/tylergannon/gimbal/internal/observation" {
			observation = pkg
		}
	}
	if observation == nil {
		return fmt.Errorf("appgen: observation package was not loaded")
	}
	snapshot, err := skgo.NewPackageSnapshot(load, roots...)
	if err != nil {
		return err
	}
	output, err := observationOutput(load, observation, roots...)
	if err != nil {
		return err
	}
	cfg.Packages = []*skgo.PackageSnapshot{snapshot}
	if err := skgo.Run(cfg); err != nil {
		return err
	}
	// SKGO prunes frontend artifacts it did not produce. Publish Gimbal's
	// separately owned observation projection afterward, using that same snapshot.
	dir := filepath.Join(root, "web/src/lib/observation")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, file := range output.Files {
		if err := os.WriteFile(filepath.Join(dir, file.Name), file.Content, 0644); err != nil {
			return err
		}
	}
	return nil
}

// observationOutput projects caller-owned symbols without loading the package again.
func observationOutput(load *packages.Config, observation *packages.Package, roots ...*packages.Package) (typescript.Result, error) {
	projection, err := grammar.FromPackages(load, observation, roots...)
	if err != nil {
		return typescript.Result{}, err
	}
	var types []grammar.Root
	for _, name := range []string{"InterviewRow", "TurnRow"} {
		object := observation.Types.Scope().Lookup(name)
		if object == nil {
			return typescript.Result{}, fmt.Errorf("appgen: observation type %s is missing", name)
		}
		types = append(types, grammar.Root{Type: object.Type()})
	}
	defs, _, err := projection.Lower(types)
	if err != nil {
		return typescript.Result{}, err
	}
	return typescript.Generate(defs, typescript.Options{})
}
