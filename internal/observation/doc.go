// Package observation holds one run's store: the six tables of facts as Go
// maps, the roll-ups computed from them, one transcript projection per turn,
// and the bounded subscribers the web server streams to.
//
// It imports neither the root workflow package nor web. The root package
// publishes into a store; web reads and subscribes to one. A store is found
// through a Registry the web runtime puts in its context, so there is no
// global map of runs and no new workflow-facing name.
//
// The store is the producer. It never blocks on a subscriber, and it writes
// each table's JSON file beside the run log as that table changes, so a
// finished run's directory is self-contained and greppable with jq. The log
// is still the input: a run this process never saw is replayed through the
// same fold, which is how a directory holding only logs gets its files.
package observation
