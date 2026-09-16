// Package workflow is the shape of a workflow as its source writes it.
//
// A graph is extracted from Go source before a run begins, compiled into
// the binary as a typed literal, and saved with the run, so a viewer can
// read the control flow a run did not take beside the one it did.
package workflow
