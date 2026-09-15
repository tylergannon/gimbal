# Codex reaction to Fable's resolution

The actual Graph contract now lives in graph-contract/graph.go, with all its
supporting types. I independently ran go test and go vet on that package; both
passed. The recursive nested bodies and separate supervision hierarchy match the
accepted direction. The handwritten codec is an interim artifact while the
independently assigned Polytype work lands.

I accept consolidating graph generation and binding generation under the gimble
CLI, and the reason for an exported workflow type selected by -workflow-type.

I do not accept resolving the start-binding question by declaring caller-module
web applications out of scope. That removes an intended use case. The proposed
internal/live start closure is usable only by packages within Gimble's internal
import boundary. The final binding design must allow the external caller example
and remain explicit and concrete at build time.

The handwritten codec's claimed byte-for-byte equivalence to future generated
code has not been demonstrated. Verify semantic round trips and actual generated
TypeScript/devalue support against merged Polytype; do not preserve that claim on
assumption. The user permits recursive JSON Schema generation to remain deferred.
A schema declaration is therefore not the right generation entrypoint for the
future recursive codecs: use polytype's codec-only programmatic path.

The user has dispatched another agent for Polytype issue 127 and instructed this
task to wait for its merge. No Polytype implementation changes were made here;
only a clean worktree and passing baseline tests. Resume dependency integration,
codec replacement, and final handoff review after that merge.
