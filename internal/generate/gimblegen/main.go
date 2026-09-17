// Command gimblegen writes a workflow's generated graph and run command.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/tylergannon/gimble/internal/generate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "gimblegen:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("gimblegen", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
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
		return errors.New("-entry and -name are required")
	}
	return generate.Source(".", *entry, *name, *output)
}
