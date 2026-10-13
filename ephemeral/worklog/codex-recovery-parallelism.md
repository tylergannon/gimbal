# Codex recovery fixture scheduling

2026-10-13. Base `4b7342cd`; measured with the simultaneous context-render improvements already present in this worktree. Only `codex/recovery_test.go` changes for this task.

The seven recovery cases each own their daemon/server, adapter, mutex-protected state, effect file, project directory, and complete three-outcome implementation run. They share only a read-only test environment setting and immutable registry data. Set `TYPESAFE_API_KEY` once in the nonparallel parent, then allow child cases to run through Go's bounded parallel test scheduler. Create each fixture and its deadline after `t.Parallel`, so queued cases do not consume their timeout.

All recovery modes, transport paths, three outcomes, planning/validation/effect counts, and exact no-replay assertions remain. No production durability, timeout, or observer behavior changes. The fixture already sets recovery delay to zero; multi-second cost comes from actual workflow/observation work rather than a mandatory recovery timer.

Focused measurements, existing build caches and uncached test execution:

| Scheduling | Parent actual elapsed | Package elapsed | Outcome |
|---|---:|---:|---|
| Original serial cases, `-parallel=2` | 5.347 s | 5.703 s | All seven passed |
| Parallel cases, `-parallel=2` | 4.654 s | 4.934 s | All seven passed |
| Parallel cases, default Go parallel bound | 4.066 s | 4.332 s | All seven passed |
| Parallel cases, race detector, `-parallel=2` | Not separately recorded | 6.819 s | Passed without race reports |

The two-slot comparison reduced observed parent elapsed about 13%; the default-bound run about 24%. This is a single before/after pair, not a claim about every machine or load condition. Each parallel child individually takes longer under I/O contention; the gain is overlapping independent cases, not eliminating disk work. Parent `go test -json` reports `Elapsed: 0` for parallel descendants, so actual parent elapsed was calculated from its `run` and final `pass` timestamps. Old 14.41-second full-suite measurements have different contention/source conditions and are not the baseline here.

Commands: `go test -count=1 -parallel=2 -json ./codex -run '^TestImplementRecoversActiveCodexTurnWithoutReplayingOutcomes$'` before/after; the same command without `-parallel=2` for the default bound; `go test -race -count=1 -parallel=2 ./codex -run '^TestImplementRecoversActiveCodexTurnWithoutReplayingOutcomes$'` for concurrency verification. No whole-repository tests or builds ran for this change.
