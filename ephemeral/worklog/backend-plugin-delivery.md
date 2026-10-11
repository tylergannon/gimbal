# Backend plugin delivery

decision: The user authorized fulfillment of the final goal handoff from chat 01a128b3-d101-7cb2-9457-72b859a8ceca, not merely the earlier bounded PR 444 plan. The two pinned handoff files govern scope; implementation belongs in Gimbal.
correction: PR 444's remote tasks do not establish remote workflow placement or independently built workflow/backend plugins. Completion needs the composed external-project path and human controls.
correction: User narrowed this delivery: leave pauses for human input out for now. Live proof must not depend on an interview or human answer; retain observation, steering, cancellation, restart, service and artifact proof.
friction: The first review was accidentally dispatched before the design artifact existed and returned invalid review request. It is not a review verdict. Future review launch must verify the target exists before dispatch.
correction: User requires no redundant Go/JSON round trips. The Go output contract supplies JSON Schema; the harness adapter validates agent JSON before returning those bytes to the workflow, which converts JSON to its concrete Go struct. Transport envelopes do not justify decoding and re-marshalling the result payload.
friction: Design review proposed a form map that skgo rejects. Verified named-struct slice form input through actual skgo generation (exit 0); its runtime Go SubmitForm encoder limitation is separate and does not govern browser submissions. CLI uses central control transport. Removed temporary probe source.
decision: Independent design review round05 reported only nitpicks. Clarified typed-decode failures use existing bounded re-asks and invalid-output classification is exported. User then requested a check-in before implementation; implementation has not begun and is held pending that conversation.

decision: User authorized implementation from the reviewed design in the linked chat. Resumed existing worktree; split coherent authoring/catalog, execution/observation and JSON adapter boundaries. Root owns Docker receipt release and workflow placement/routes integration.

Execution seam: public backend DTOs remain standard-library-only. Runtime task
routing now resolves one connection for runner/model bindings; native task events
and authoritative result reads run independently. Command captures live at B and
are streamed to A before acknowledgement. Explicit task bytes and only runtime
Complete-value artifact markers receive B-local file paths. A missing event
source emits a gap while execution/result/control remain available. Cleanup uses
the retained connection even after scope cancellation. Retention requires native
release; compact expired task markers are removed after the bounded expiry window.

Checks: taskdaemon/runner/observation/effects/backend focused race suite passed.
Package tests exercise useful HTTP service read and scope release, explicit file
bytes in an independent execution environment, successful result during event
outage, expired payload distinction, remote source gap replacement, committed
suffix folding, and unavailable observation preserving running status. These are
package behavior checks; they do not establish independent-provider or visual UI
proof.

- decision: Native output validation occurs once in each adapter; Generate retains typed decode and bounded reasks. Custom ValidateJSON is no longer an Output constraint; planner semantic acceptance checks the decoded plan.
- friction: encoding/json compacts nested json.RawMessage when encoding transport envelopes -> carry native Output as []byte in backend.TurnResult so whitespace and invalid bytes survive transport. Decode only the envelope and usage at the remote adapter.
- friction: skgo generate must run from internal/skgo as its existing directive specifies; invoking from repository root generated malformed module import paths. Keep generation single-owner because it replaces linked trees and bindings.

- Event delivery proof exposed a swallowed synthetic gap: the remote harness's event stamper requires native session identity. The bridge now stamps runtime observation events and projection carries unavailable/restored state; an actual Run/Generate HTTP test also exercises steering during the outage. A healthy empty event poll cannot establish observation restoration.
- Native Go plugins require rebuilding after shared-package edits, including effects and observation changes. An initial Docker conflict probe hit the expected ABI mismatch before admission; repeat against the final built binary, not a stale host.
- Native skgo form fields must come from form.fields.as(), which supplies per-form suffixes and boolean coercion. Indexed plain HTML names alone are not the real supported no-JS path.

Live execution exposed a real source seam defect: an unchanged observation
cursor returns `Snapshot: null`; decoding into RawMessage produces four bytes,
which had incorrectly triggered replacement with an empty snapshot. Source
refresh now distinguishes JSON null from an actual replacement, with a package
regression test. This erased central's run state and broke run-page SSR and CLI
run discovery despite valid A observations; the failure was not execution death.

Event gap propagation required runtime identity rather than forwarding a bare
transport notice through native event stamping. The harness bridge stamps
runtime observation events, and the durable turn projection now marks event
unavailability separately from terminal execution/error. A focused race test
covers gap, control delivery, received-event resumption, and successful result.

- Final Runs browser gate exposed cached terminal snapshots after route moved to shared Registry. List loads now retain published live stores and read fresh terminal checkpoints with remote routing/placement preserved. The publication-boundary integration test and browser refresh scenario cover both sides. Route Go is copied by skgo into internal/skgo/links; regenerate those copies before compiling. Focused skgo generation deletes independent workflow type output, so restore workflow Polytype projection before frontend checks.

- Live shared-form proof found enhanced submission rejected before skgo: projectRequest treated every remote endpoint as project-owned, while the workflow catalog form is central and chooses its project in the typed body. Admit the generated launch.remote.ts endpoint with central Owner/Projects context, retain project guards for other remotes. Actual Instance enhanced binary request regression passes; native query-preserving action test also passes.

Default-local shared form execution exposed another real seam defect: service
fulfillment had required an HTTP task endpoint, despite the local default using
native direct tasks. The local path now delegates to existing scope-owned native
Service execution and returns loopback in that workflow environment. A focused
race test reads exact bytes from the real foreground HTTP service and verifies
scope exit releases it. Remote fulfillment still requires declared topology.

friction: Real remote follow exposed absent Source snapshots encoded as JSON null. Treating null as replacement erased central projection. Source-null regression now preserves rows; real follow subsequently completed.
friction: External service workflow exposed generator omission of FulfillService and accidental persistence of private source-inspection graph pointer markers. Recognize the existing service node and independently extract persisted graph; external module generation/build/regeneration now exercises scoped services.
friction: Real enhanced shared-form submission returned404 although native submission passed. Instance routing required project-page Referer for all remotes; admit the generated launch form in central owner context. Integration now exercises enhanced routing plus query-bearing native action.
friction: Browser selected first project while form state lacked explicit fallback. Set the form select default; blank validation followed by goal-only retry now works.
friction: Default local shared-form service launch failed because FulfillService required a backend endpoint. Use existing native scope-owned Service for the default local path, no extra listener or task process. Real HTTP test consumes bytes and confirms release.
friction: Full pre-commit Go suite found test teardown racing the independently spawned runner's final durable writes. The CLI test now waits the actual runner lifetime lock and instance shutdown; repeated focused test passes.
decision: CLI follow retains a stale central socket across restart, while fresh identity/control lookup works. Keep this client reconnect quirk outside the backend delivery and track it in GitHub issue445; do not claim continuous follow reconnect proof.
