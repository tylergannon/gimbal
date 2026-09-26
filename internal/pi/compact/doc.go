// Package compact ports pi's context compaction
// (packages/coding-agent/src/core/compaction, upstream d6af72e1).
//
// It holds the pure compaction logic: token estimation, cut-point selection,
// conversation serialization, summary generation through an injected provider
// call, and branch summarization. Session-manager I/O stays with the caller:
// these functions take session entries and return a proposed result for the
// session to commit. A cancelled or failed summarization never installs a
// partial history.
package compact
