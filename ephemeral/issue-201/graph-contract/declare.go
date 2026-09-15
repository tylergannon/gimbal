//go:build jsonschema

package workflow

// This file is the polytype declaration for the graph, following schema.go in
// the root package. It is compiled only under the jsonschema build tag, which
// also excludes codec.go, so the handwritten owner codecs never collide with
// generated ones.
//
// Pinned polytype v1.0.0 rejects recursive types (typegrammar/validate.go
// reports "cyclic type: back edge"), so running the generator against this
// package fails today; the exact output is recorded in
// ephemeral/issue-201/graph-contract/polytype-attempt.txt. Once polytype
// accepts recursive types, delete codec.go, add the usual
// `//go:generate go tool polytype --validate` directive, and the generated
// owner codecs replace the handwritten ones with the same wire shape.

import (
	"encoding/json"

	"github.com/tylergannon/polytype"
)

func (Graph) Schema() json.RawMessage     { panic("not implemented") }
func (Graph) ValidateJSON(_ []byte) error { panic("not implemented") }

var (
	_ = polytype.Declare(Graph.Schema)
	_ = polytype.SealedUnion[Operation]("kind", polytype.Snake)
)
