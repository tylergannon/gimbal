# Run manifests do not record what was run: no argv, binary, pipeline source, or graph hash

URL: https://github.com/tylergannon/gimble/issues/71
State: closed
Milestone: None
Updated: 2026-09-08T15:45:45Z

## The gap

A run directory cannot say what it ran. `manifest.json` records only:

```
control_socket  goal  id  name  started_at  workdir
```

There is no argv, no executable path or hash, no record of whether the pipeline came from a file or from the embedded catalogue, and no hash of the resolved graph.

## Why it matters

It makes run artifacts unusable as evidence. Adversarial review of the v0.9.0 verification runs landed on this as its strongest objection, and it is correct:

> The claimed inheritance proof is circular: the installed binary currently shows an inner loop with no checklist, while the run shows a resolved inner path — but the run never proves it used that embedded definition. A supplied graph with the path hardcoded would produce the same manifest, stages, prompts, validations, and ledgers.

So "I ran `tractor run chapter-loop` and it worked" is not checkable after the fact. Nothing distinguishes a run of the embedded workflow from a run of a local file, or from inline YAML, or from a copy since deleted. `name` in the manifest is the pipeline's own `name:` field, which any file can claim.

The same objection reappears for every claim a run directory is offered as evidence for, which is most of them.

## Fix

Record provenance in `manifest.json` at run start:

- `argv` as invoked
- the executable's path and, ideally, its build version
- how the pipeline was resolved: `builtin:<name>` or `file:<path>` or `inline`
- a hash of the resolved graph after parsing, so the manifest pins what actually ran rather than what was on disk at the time

None of it needs a schema change to the graph language, and all of it is known at the moment the run starts.

## Why now

This is the cheapest thing that makes proof artifacts self-evidencing, which is a prerequisite for gating releases on them (#69) and for treating a stored run as durable evidence at all. Without it, every proof needs a human to vouch for how it was produced.

## Related

Found by the adversarial review that also produced #70. Prerequisite for #69.

🤖 Generated with [Claude Code](https://claude.com/claude-code)

