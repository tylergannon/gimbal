//go:build ignore

// Command gen generates this package's JSON codecs.
//
// It asks polytype for Go JSON codecs and nothing else. A Graph is never sent
// to a model as structured output, so it needs no JSON Schema, and JSON Schema
// is the one output that cannot express the recursion in this model.
//
// This is the programmatic path: codegen.Gen takes the configuration as a
// value, so the declaration does not need an entrypoint the way a scanned
// declaration file does.
package main

import (
	"log"

	"github.com/tylergannon/polytype"
	"github.com/tylergannon/polytype/codegen"

	workflow "github.com/tylergannon/gimble/ephemeral/issue-201/graph-contract"
)

func main() {
	err := codegen.Gen(
		polytype.Compose(
			polytype.Declare[workflow.Graph](),
			polytype.SealedUnion[workflow.Operation]("kind", polytype.Snake),
		),
		codegen.GoJSON(),
		codegen.Target("."),
		codegen.Pretty(),
	)
	if err != nil {
		log.Fatal(err)
	}
}
