// Package workflow is the settled type contract for issue 201's workflow graph.
//
// This directory is the handoff copy of the future production package
// github.com/tylergannon/gimble/workflow. It compiles on its own inside the
// gimble module, and its test proves the JSON round trip of nested bodies and
// nested supervision. Sol moves these files to workflow/ when implementing;
// nothing here is imported by production code yet.
//
// Shape decisions, all recorded in ephemeral/issue-201/graph-model.md:
//
//   - The Go model is the natural recursive one. Ordered bodies are
//     []Operation, subgraphs hold bodies, and a Supervisor holds the
//     supervisors of its own look turns. Nothing is flattened into reference
//     tables.
//   - Operation is a sealed interface. Its variants are the same-package
//     structs declaring the unexported marker: AgentCall, Command, Set,
//     SetJSON, Exit, and the five Subgraph forms Sequence, Condition, Loop,
//     Scope, and Group. Subgraph is the narrower sealed marker for the five
//     forms that contain ordered bodies.
//   - Supervision is a separate, unordered hierarchy beside the watched call.
//     It hangs off the construct that owns the execution scope of that call
//     (Graph, Scope, Loop, or a Group child), not off the ordered body.
//   - Sessions are declared once per Graph with their owning scope. Calls and
//     supervisors reference a session by static ID; repeated references never
//     duplicate a conversation.
//   - Every static site carries an ID and a source anchor. IDs are stable
//     across regeneration of unchanged source; they are not runtime instance
//     IDs.
//
// Wire format. The handwritten codec in codec.go produces exactly the shape
// polytype v1.0.0 generates for a sealed union with
// SealedUnion[Operation]("kind", polytype.Snake): each operation is one JSON
// object whose "kind" property is the snake_case Go type name and whose other
// properties are the struct's own fields. Nil slices encode as [] and unknown
// or foreign fields are rejected on decode. Pinned polytype rejects recursive
// types, so the codec is handwritten until polytype accepts them; declare.go
// keeps the polytype declaration that will replace it. See graph-model.md,
// "Encoding decision", for what still does not cross the TypeScript/devalue
// boundary.
package workflow
