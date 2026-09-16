package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/tylergannon/gimble/internal/graph"
)

// runGen writes the graph and the gimble run subcommand of one workflow,
// read from the source in the current directory, as one Go file in that
// package. It is what a workflow's generate directive runs.
func runGen(args []string, stderr io.Writer) error {
	flags := flag.NewFlagSet("gimble gen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	entry := flags.String("entry", "", "the workflow's entry function")
	name := flags.String("name", "", "the workflow's name, which is also the run's name")
	output := flags.String("o", "workflow_gen.go", "the file to write")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *entry == "" || *name == "" {
		return errors.New("gen: -entry and -name are required")
	}
	return graph.Source(".", *entry, *name, *output)
}
