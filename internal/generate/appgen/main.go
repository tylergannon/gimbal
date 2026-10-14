// appgen composes Gimbal's stock source, SKGO bindings and observation types.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/generate"
	"github.com/tylergannon/polytype"
	"github.com/tylergannon/polytype/codegen"
	"github.com/tylergannon/polytype/goload"
	skgo "github.com/tylergannon/skgo/generate"
)

type remotes []string

func (r *remotes) String() string         { return fmt.Sprint([]string(*r)) }
func (r *remotes) Set(value string) error { *r = append(*r, value); return nil }

func main() {
	if len(os.Args) > 1 && os.Args[1] == "dev" {
		if err := development(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}
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
	var session goload.Session
	return runWithSession(root, cfg, &session)
}

func runWithSession(root string, cfg skgo.Config, session *goload.Session) error {
	// Stock publishes authored route inputs before SKGO selects its source.
	if err := generate.Stock(root, session); err != nil {
		return err
	}
	cfg.Session = session
	if err := skgo.Run(cfg); err != nil {
		return err
	}
	// Each producer owns its outputs and checks its persistent result before
	// asking the invocation-scoped session for package work.
	return observationOutput(filepath.Join(root, "internal/observation"), filepath.Join(root, "web/src/lib/observation"), session)
}

func observationOutput(sourceDir, outputDir string, session *goload.Session) error {
	return codegen.Generate(observationConfiguration(), codegen.Options{
		Session: session, TargetDir: sourceDir, TypeScript: &codegen.TypeScriptOptions{Dir: outputDir},
	})
}

func observationConfiguration() polytype.Configuration {
	return polytype.Compose(
		polytype.DeclareType(polytype.TypeSpec{PackagePath: "github.com/tylergannon/gimbal/internal/observation", Name: "InterviewRow"}),
		polytype.DeclareType(polytype.TypeSpec{PackagePath: "github.com/tylergannon/gimbal/internal/observation", Name: "TurnRow"}),
	)
}
