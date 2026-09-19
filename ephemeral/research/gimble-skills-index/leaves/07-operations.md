# Operations: the Gimble programmer journey

authority: repository source at manifest commit `db40543c015d7ce102c50e69cb9ea79a47b64e67`.
currentness: sources were read from the manifest's `07-operations` segment on 2026-09-18; implementation and comments are current at that commit, while `docs/web-app.md` explicitly distinguishes shipped, designed, and later UI features.
audience: an agent authoring the future Gimble operations/runs skill, and the reader-11 agent consuming CLI help and workflow-packaging findings.

## Core journey

1. Discover the installed binary's surface with `gimble --help`, then discover built-in workflows with `gimble run --help`; the current root command describes itself as the project runtime/web app and registers `run`, `runs`, `watch`, `steer`, and `run-prompt` (`cmd/gimble/main.go:L93-L119`).
2. Choose a workflow and inspect its own help before invoking it. The operations skill says `gimble run --help` lists built-ins and each workflow's `--help` lists inputs and model flags (`.agents/skills/gimble-runs/SKILL.md:L10-L13`). The only workflow visibly registered by this segment is `review`, supplied through `review.Command(workflowDefaults())` (`cmd/gimble/workflows.go:L23-L31`); its default role map currently contains `code-review: gpt-5.6-luna` (`cmd/gimble/defaults.json:L1`).
3. Invoke with an absolute project directory and an explicit goal, keeping the process alive while observing or steering. The documented form is `gimble run review --work-dir /abs/repository --goal "Review the parser changes" --no-web` (`.agents/skills/gimble-runs/SKILL.md:L12-L21`). The root defaults are loopback port 8080, optional Unix socket, and web enabled unless `--no-web` is supplied (`cmd/gimble/main.go:L122-L143`).
4. Discover active runs with `gimble runs --work-dir /abs/repository`, choose the returned run ID, then follow it with `gimble watch <run-id> --work-dir /abs/repository` (`.agents/skills/gimble-runs/SKILL.md:L23-L34`). The actual JSON list contains id, name, status, and owning runtime PID, sorted by run ID (`cmd/gimble/runs.go:L35-L44,L76-L95`).
5. Read the watch stream as a JSON snapshot followed by observation changes. `watch` requires exactly one run argument, contacts the owning live runtime, and streams `/api/runs/<id>/events`; a missing live owner is an error (`cmd/gimble/runs.go:L60-L73,L97-L115`). The CLI converts the SSE opening snapshot to a JSON object with `type: "snapshot"` and wraps later events as `{type,data}` (`cmd/gimble/runs.go:L152-L196`).
6. Identify an active target from the snapshot: a turn with no end time is active, and its session ID must be copied exactly. Role names and turn IDs are not session IDs (`.agents/skills/gimble-runs/SKILL.md:L30-L34`). This supports the operator story of finding live work, spotting stuck turns, and knowing whether a session currently has a turn (`docs/web-app.md:L108-L119; docs/web-app.md:L90-L94`).
7. Deliver a session steer with `gimble steer RUN --work-dir /abs/repository --session <session-id> "..."`, or target a loop planner with `--loop <loop-scope-id>` (`.agents/skills/gimble-runs/SKILL.md:L36-L45`). The command enforces exactly one of `--session` and `--loop`, rejects a blank message, and requires exactly `RUN MESSAGE` positional arguments (`cmd/gimble/steer.go:L17-L30,L68-L71`).
8. Interpret delivery separately from effect. Session steering reaches the active turn only; `landed: false` means no turn received it and the message is not saved. Loop steering queues for the next planning decision. Neither acknowledgment proves obedience; keep watching for the resulting behavior (`.agents/skills/gimble-runs/SKILL.md:L41-L45`). Runtime comments confirm session steering is recorded as a person steer and loop steering reaches the next planner decision whether or not a turn is running (`web/runtime.go:L256-L279`).
9. After completion, distinguish active control from historical reading. Finished run records remain under `.gimble/runs/<run-id>/`, but a finished process cannot receive steering; serve the project again to view saved runs or read the tables/transcripts (`.agents/skills/gimble-runs/SKILL:L47-L53`). The historical run-store sprint described six table files, but current authoring has eight tables (`run`, `scopes`, `sessions`, `interviews`, `turns`, `turn_usage`, `model_calls`, `commands`) and additionally maintains a durable reduced snapshot plus delta journal (`internal/observation/files.go:L13-L27`; `internal/observation/store.go:L54-L88`; `internal/observation/durable.go:L15-L30`).

## Teachings to encode

### T1 — Help-first discovery

audience: a new Gimble programmer. Teach the exact command ladder (`gimble --help`, `gimble run --help`, workflow `--help`) and explain that help is the authoritative inventory of commands actually registered. Cite `cmd/gimble/main.go:L93-L119` and `.agents/skills/gimble-runs/SKILL.md:L10-L13`. Authority/currentness: shipped Cobra registration plus the operations skill at the manifest commit. Do not list proposed web features as CLI commands.

### T2 — Workflow choice, inputs, models, and proof expectations

audience: a workflow author or operator choosing a built-in. Explain that the current segment registers `review`, defaults `code-review` to `gpt-5.6-luna`, and exposes workflow-specific input/model flags through generated command help (`cmd/gimble/workflows.go:L12-L31`; `cmd/gimble/defaults.json:L1`; `.agents/skills/gimble-runs/SKILL.md:L10-L17`). Authority/currentness: implementation is current; the exact review flags are owned by the registered workflow command and should be retrieved from its installed `--help`, not inferred here. Proof expectation: report the workflow, configured model, run ID, target, delivery result, and observed effect (`.agents/skills/gimble-runs/SKILL.md:L52-L53`).

### T3 — Project and listener defaults

audience: an operator starting a run. Teach `--work-dir` as the project selector and explain the web defaults: loopback TCP port 8080, `--port 0` for an available port, `--uds` for a Unix socket, and `--no-web` for headless execution (`cmd/gimble/main.go:L122-L143; .agents/skills/gimble-runs/SKILL.md:L19-L21`). Authority/currentness: CLI flags and runtime options are shipped. Limits: `--no-web` still leaves control, runs, and durable records available (`web/runtime.go:L80-L89`).

### T4 — Discover the exact live run

audience: an operator with one or more local runtimes. Teach that `runs` discovers `.gimble/control/*.json` hints, verifies live runtimes by querying `/control/runs`, and emits only runs that respond successfully (`cmd/gimble/runs.go:L199-L247; web/control.go:L21-L23,L168-L193`). Authority/currentness: implementation comments and code. Limit: failed or unreachable runtime queries are skipped, so an empty list is not proof that no process exists (`cmd/gimble/runs.go:L231-L247`).

### T5 — Watch is a live observation stream

audience: an operator or debugger. Teach that `watch` first locates a live owner, then emits an opening snapshot and subsequent changes from the SSE endpoint (`cmd/gimble/runs.go:L97-L115,L152-L196`). Authority/currentness: shipped CLI behavior. Explain the error boundary: a saved run without a running owner cannot be watched through this command (`cmd/gimble/runs.go:L97-L114`); historical viewing belongs to the web runtime/store path (`.agents/skills/gimble-runs/SKILL.md:L47-L50`).

### T6 — Target IDs are not interchangeable

audience: anyone steering. Teach the distinction between run ID, session ID, loop scope ID, role name, and turn ID. A session steer needs the exact active session ID; loop steering needs the loop scope ID (`.agents/skills/gimble-runs/SKILL.md:L30-L39; cmd/gimble/steer.go:L39-L46`). Authority/currentness: explicit skill and command contracts. Retrieval hint: obtain candidate IDs from the watch snapshot before issuing a steer.

### T7 — Delivery versus effect

audience: supervisors and reviewers. Teach that `landed` is a delivery fact, not a behavior claim; loop `queued` is likewise only acceptance into planner state (`.agents/skills/gimble-runs/SKILL.md:L41-L45; web/control.go:L63-L99`). Authority/currentness: runtime control response and skill contract. Proof expectation: continue `watch` and report what changed afterward; the UI story also requires a steer to be attributed and visibly landed or dropped (`docs/web-app.md:L121-L132`).

### T8 — Runtime control boundaries

audience: operators diagnosing errors. Teach that unknown or finished runs/sessions/scopes are errors, session steering targets the active turn, and loop steering is valid only for a loop still dispatching (`web/runtime.go:L256-L279`). The runtime also exposes cancellation methods for scopes and turns, but no CLI command registers them in this segment (`web/runtime.go:L282-L303; cmd/gimble/main.go:L112-L119`). Authority/currentness: runtime API comments plus actual root command registration. Mark cancellation as an API capability, not an available CLI operation.

### T9 — Results and errors in the page

audience: an operator reading live output or a reviewer reading afterward. Teach the intended page vocabulary: run header/status/error, scope task/decisions/values, session adapter/model, turn prompt/result/error/duration/usage, and transcript harness errors (`docs/web-app.md:L86-L100`). Authority/currentness: UI design record, with explicit status labels (`bare`, `partly`, `exists`, `designed`, `in progress`) indicating what is shipped versus proposed. Do not promise the entire feature table from the current page.

### T10 — Historical records and currentness

audience: a reviewer reopening a run. Distinguish the historical sprint contract—six table files, with replay when one is missing—from the current store: eight table files are written atomically, and `observation.json` plus `observation-deltas.jsonl` provide reduced durable recovery (`docs/sprints/RUN-STORE.md:L293-L335`; `internal/observation/files.go:L13-L27,L102-L123`; `internal/observation/durable.go:L15-L30,L55-L116`). Session logs are transcript input after table facts are restored; current store comments say table facts are accounting and session-log steps account for nothing (`internal/observation/store.go:L82-L88`). Authority/currentness: current Go sources override the historical sprint prose; label the sprint's six-file statements as historical.

### T11 — CLI help must teach proof limits

audience: future CLI/help authors and reader 11. Each built-in's help should state purpose, required inputs, model/default selection, outputs/artifacts, proof expectations, and limits. This is a current user priority layered on top of the shipped help inventory; the segment itself proves only that workflow-specific flags are exposed (`.agents/skills/gimble-runs/SKILL.md:L10-L13; cmd/gimble/workflows.go:L23-L31`). Mark any generated declaration/help comparison as pending reader-11's generator findings; no generator mechanism is established by these operations sources.

### T12 — Make command outputs machine-usable

audience: programmers composing local tooling around Gimble. Teach that `runs` emits one JSON object with a `runs` array, while `watch` emits newline-delimited JSON observations (`cmd/gimble/runs.go:L42-L44,L81-L95,L152-L177`). Authority/currentness: shipped implementation. The stable fields currently visible for a listed run are `id`, `name`, `status`, and `pid`; do not imply that the list includes sessions, turns, costs, or finished records (`cmd/gimble/runs.go:L35-L44,L81-L89`).

### T13 — Explain the control discovery mechanism without making it user work

audience: an operator diagnosing “no runs found.” Teach that the project directory is made absolute, `.gimble/control` contains JSON discovery hints, and malformed entries or dead runtimes are ignored (`cmd/gimble/runs.go:L199-L228`). The runtime creates and removes its control socket/discovery file with the runtime lifecycle (`web/control.go:L102-L160`). Authority/currentness: implementation comments and code. The user-facing lesson is “run the command in the right project directory and ensure the runtime is alive,” not “manually edit control files.”

### T14 — Separate browser observability from CLI control

audience: a programmer deciding which surface to use. Teach that every runtime serves the project page and can expose a loopback TCP listener, UDS, or no web, while the local control socket remains available for `runs`, `watch`, and `steer` (`web/runtime.go:L41-L89,L100-L153`). Authority/currentness: runtime implementation. The page is the richer visual surface; the CLI is the exact, scriptable control path (`docs/web-app.md:L3-L10; cmd/gimble/main.go:L112-L119`).

### T15 — Preserve the scope key as the organizing identifier

audience: operators reading nested work and future UI/help authors. Teach scope keys such as `lap.3/bakeoff.1/attempt.2` as the containment, breadcrumb, usage grouping, and source join (`docs/web-app.md:L78-L84`). Authority/currentness: web design vocabulary. Use this alongside exact session IDs for steering; a scope key is a loop target only when the target is a currently dispatching loop (`web/runtime.go:L268-L273`).

### T16 — Historical truth and proof

audience: reviewers making claims about a completed run. The historical sprint proof names six table files and no `observation.json` (`docs/sprints/RUN-STORE.md:L272-L291`; `docs/sprints/RUN-STORE.md:L504-L538`); current proof must account for eight table files and the durable snapshot/delta path (`internal/observation/files.go:L13-L27,L121-L123`; `internal/observation/durable.go:L15-L30,L55-L116`). In either era, compare observed page totals before/after a live run and after restart; file existence alone is not behavioral proof.

### T17 — Error and cancellation limits

audience: supervisors deciding what can be controlled safely. Teach that session and loop control errors name unknown/finished runs, unknown sessions, or non-dispatching scopes, while `KillScope` and `KillTurn` are runtime methods rather than registered CLI commands (`web/runtime.go:L256-L303; cmd/gimble/main.go:L112-L119`). Authority/currentness: current API comments plus command registration. The future skill should say exactly which operation is available from which surface.

## Gaps, conflicts, and retrieval hints

- The CLI currently registers `run`, `runs`, `watch`, `steer`, and `run-prompt`, but the web design lists interrupt/cancel and richer run controls as designed features; do not teach those as registered commands without another source (`cmd/gimble/main.go:L112-L119`; `docs/web-app.md:L96-L100,L121-L130`).
- `docs/web-app.md` calls the runs list live and past, while the `runs` CLI is explicitly “runs in progress” and the control endpoint filters through `InProgress` (`docs/web-app.md:L86-L89`; `cmd/gimble/runs.go:L46-L57`; `web/control.go:L168-L188`). Teach these as separate surfaces: CLI discovery of active runtimes versus page/store history.
- The web document proposes URL links to scope/session/turn and richer graph views, but says these are not yet decided or are later; retrieval should preserve those status labels (`docs/web-app.md:L53-L65,L86-L101,L134-L150`).
- For operations examples, retrieve `.agents/skills/gimble-runs/SKILL.md`, `cmd/gimble/main.go`, `cmd/gimble/runs.go`, and `cmd/gimble/steer.go` first. For control semantics, add `web/control.go` and `web/runtime.go`; for historical truth, add `docs/sprints/RUN-STORE.md`; for user-facing goals, add `docs/web-app.md`.
