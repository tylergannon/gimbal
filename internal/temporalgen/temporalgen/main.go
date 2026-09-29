// Command temporalgen emits the bounded experimental Temporal target.
package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/tylergannon/gimbal/internal/temporalgen"
)

func main() {
	dir := flag.String("dir", ".", "authored Go package")
	entry := flag.String("entry", "", "authored entry function")
	name := flag.String("name", "", "Gimbal workflow name")
	output := flag.String("output", "", "generated Go file in the instrumented backend package")
	flag.Parse()
	if *entry == "" || *name == "" || *output == "" {
		log.Fatal("-entry, -name and -output are required")
	}
	path, err := filepath.Abs(*output)
	if err == nil {
		err = temporalgen.Source(*dir, *entry, *name, path)
	}
	if err != nil {
		log.Fatal(err)
	}
}
