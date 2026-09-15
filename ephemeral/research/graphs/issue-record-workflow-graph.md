Record the generated workflow shape with each run so a finished run can show the program it actually used, even after the workflow source changes. #201 generates the graph; this issue connects that output to the run record. Tyler's sequencing decision: this must work before building the viewer in #173.

## Result

- A workflow supplies the graph generated for its build. The run preserves that graph and its source/build identity in its run directory, using #201's format rather than a second graph model.
- The observation API makes that recorded graph available for both live and finished runs. Reading an old run does not regenerate its graph from today's checkout.
- Missing graph data is represented honestly. Existing runs without a graph remain readable; they are not assigned the current workflow's graph.
- The builtin sprint and a caller-module example demonstrate the same path. Keep the integration small and explicit; agree the attachment mechanism with #201's implementer rather than inventing runtime source inspection.

## Proof

Run a workflow with graph A, then change and regenerate its source to graph B. The original saved run still returns A after restart, and a new run returns B. Verify the recorded graph and its identity through the observation API without building a viewer. Also show that a run without a supplied graph remains readable and reports no graph.

## Sequence and boundaries

#201 → this issue → #173. Review the generated format with #201 while extraction is in flight; finish this integration once that format is usable. Viewer implementation starts after graph recording is demonstrated.

This records static program shape. It does not claim exact source-site correlation for every runtime event, instrument command execution, or add viewer layout. No reflection or runtime.Caller. Closed legacy issue #71 concerned the previous pipeline system and is not an implementation of this contract.
