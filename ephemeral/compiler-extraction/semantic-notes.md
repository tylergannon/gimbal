# Stage 2 semantic extraction notes

decision: Keep the experiment private. Operations now dispatch to a scope-attached runtime implementation; the single OpenRun bootstrap callback still breaks the root/leaf import cycle. The private transport boundary retains opaque Session handles and JSON task/plan values; shared root helpers use the actual Session/Task/plan types. Publishing typed root entry points remains Stage 3 work.

decision: Physical immutable objects use one content-addressed namespace, while LocalDir is a separate materialization root. Existing filesystem callers select Root; the memory-backed object test proves no object resolution depends on a filesystem layout. Context artifact observations name context/objects/<hash> for the configured host resolver.

correction: Check records must derive from CommandStarted/CommandEnded, including resolved absolute workdir. Duplicate local keys are rejected before execution. Publication errors retain the supplied snapshot, while a command execution failure still publishes its evidence.

decision: Runtime session reachability is checked for Generate/planner dispatch/Fork. Finish refuses open children and duplicate cleanup; the consumer still owns lease admissions and joining concurrent work. Snapshots remain independent of live ancestry and explicit empty input cannot fall back to ambient values.

correction: Independent Astra validation proved that returning eventResult errors from successful context publication and plan recording stopped authoritative execution. Both now emit through run.event; diagnostic failure stays in run.recordingError and final Run error. A regression first failed with zero harness turns, then passed with the subsequent turn completed and the recording failure retained. Required object-store publication errors remain operation failures.

correction: Review findings 2/3 were reproduced before repair: a second InitializeContext accepted a different object backend and materialization directory, and a blocked object Put prevented an unrelated Set. Store configuration now occurs once during InitializeContext; private runtime operations take only snapshot references and use that configured store. Reinitialization fails without comparing Objects implementations, including uncomparable implementations. The specimen emitter and callers use this same entry boundary.

correction: Context publication prepares immutable objects and materializations outside the scope mutex, then rechecks duplicate keys and ended state before installing observations. Focused regressions cover unrelated Set progress, a competing duplicate write, and scope finish while Put is blocked. Required storage failures still fail the operation; diagnostic writes still use run.event.

decision: Private Store operations carry caller context to physical Put/Get. Ordinary canceled publication stops through a cancellation-aware object backend. Check deliberately detaches validation/publication from command cancellation so its canonical canceled-command evidence still produces a snapshot; a regression retains that existing behavior. Durable run artifact publication and the specimen's context-free payload codec remain detached. This does not claim that a backend ignoring its supplied context can be forcibly interrupted.
