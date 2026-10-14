// Command gimbalgen writes a workflow's graph and optional application command.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/tylergannon/polytype/goload"

	"github.com/tylergannon/gimbal/internal/generate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "gimbalgen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("gimbalgen", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	batch := flags.String("batch", "", "JSON file of source inspection requests")
	entry := flags.String("entry", "", "the workflow's entry function")
	dir := flags.String("dir", ".", "Go package directory to analyze; the workflow is never executed")
	name := flags.String("name", "", "the workflow's name, which is also the run's name")
	output := flags.String("o", "workflow_gen.go", "output: .go graph registration, .html source viewer, or .json inspection data (inspection does not execute the workflow)")
	mermaid := flags.String("mermaid", "", "optional .mmd file for the workflow diagram")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *batch != "" {
		raw, err := os.ReadFile(*batch)
		if err != nil {
			return err
		}
		var requests []generate.InspectionRequest
		if err = json.Unmarshal(raw, &requests); err != nil {
			return err
		}
		root, err := os.Getwd()
		if err != nil {
			return err
		}
		var session goload.Session
		return json.NewEncoder(os.Stdout).Encode(generate.InspectBatch(root, requests, &session))
	}
	if *entry == "" || *name == "" {
		return errors.New("-entry and -name are required")
	}
	return generate.Source(*dir, *entry, *name, *output, *mermaid)
}
