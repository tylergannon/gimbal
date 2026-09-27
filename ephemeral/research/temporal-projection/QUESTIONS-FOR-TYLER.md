# Decisions that minimize the code

Each answer removes work. Recommended answer first.

1. **Is Temporal a requirement or a means?** Step 0 (whole run in a pod)
   needs no Temporal; Temporal earns its place at step 1 (per-turn
   activities, retries on another pod, backlog scaling). If a means:
   build step 0 on a plain Kubernetes Job and defer Temporal until a run
   needs to outlive a worker. If a requirement: step 0 is itself one
   Standalone Activity per run, same code. (round2/r1-1, r1-4)

2. **Name the commands seam.** `RunCommand`, `Check`, `Service` are
   hardwired to `os/exec`. Remote execution needs an interface beside
   `HarnessAdapter` (the audit calls it a command runner). AGENTS.md
   reserves new exported names for you. Alternative with no new name: the
   whole workflow body runs on the worker pod (step 0), and only cross-pod
   fan-out waits. (round1/gimbal-internals-audit §1.1)

3. **Unit of placement: pod per run.** All of a run's sessions, workdir,
   services, and supervisors on one pod, one task queue per run. Keeps
   today's semantics exactly (fork-in-same-workdir, `Service` scope,
   `WalkDir` over agent output). With per-turn checkpoints of harness
   state plus workdir delta, a replacement pod can continue a run; decide
   whether the checkpoint store is a per-run EFS volume (no uploads) or a
   bucket (cold, cheap, needs upload per turn). (round2/r1-2, r1-2b)

4. **Orchestrator location: in the pod (step 0) then the shared instance
   (step 1).** Decides event transport: shared volume tailed by the
   instance, or records pushed over HTTP. Recommended: HTTP push, since the
   instance already owns `run.jsonl` and the page. (round1/infra F24-25,
   round2/r1-3)

5. **Steer and supervision stay mid-turn.** Supervisors on the orchestrator
   unchanged; steer and cancel as activities on the run's queue (not
   signals, not direct HTTP). The alternative (queue until next turn)
   silently changes `WithSupervisor` from watcher to reviewer. (round2/r1-3, r1-6)

6. **Fix the latent stock-workflow bugs first.** `validateproduct.go:122,232`
   raw `exec`; `os.Executable()` in `researchdocument` and `pyramidsummary`;
   `implementation.go:71` unrecorded file read feeding `Iterate`. Proposed
   lint GIMBAL110 to 113 would catch a sixth. Yes or no on adding those
   rule names. (round2/r2-2 §2-3, r1-4 §2)

7. **Temporal Cloud, one namespace, task queue per project.** Self-hosting
   costs 20x more at this volume and runs Postgres for you. Per-project
   Temporal namespaces fight Temporal's own guidance; per-project
   Kubernetes namespaces are fine. (round2/r1-5 §6, round1/temporal-go-sdk F9)

8. **Configuration surface: GitHub App install plus a repo file.** The
   install screen picks repos, webhooks report changes, `AGENTS.md` (or one
   Gimbal file) holds setup. The only plausible UI is egress policy. Say
   whether egress can be a repo file too. (round1/infra table)

9. **Threat model: your own repos, your own harnesses?** If yes, plain
   `runc`; Kata or gVisor only for untrusted contributor code. (round1/infra F13)

10. **Repo cache: EFS bare clone plus worktree per run, or fresh clone per
    run?** EFS removes node affinity; its small-file latency is
    unbenchmarked. Fresh clone is simpler and fits pod-per-run. (round1/infra F18)

11. **Consumer templates.** Under Finalist 1 the template is worker main,
    queue naming, and manifests; under Finalist 2 it is the emission half
    (35 to 45% of a generator). Confirm the first is what you meant by
    "templates provided by consumers". (round2/r2-2 §5)

12. **Record the native session id.** One field on `SessionCreated` or
    `TurnStarted`; cheap now, required for any resume later. (round2/r1-4 §3)
