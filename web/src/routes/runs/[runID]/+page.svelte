<script lang="ts">
  import type { Graph } from "#lib/workflow/types.js";
  import {
    RunObservation,
    type ConnectionState,
    type RunSnapshot,
    type TurnRow,
  } from "#lib/observation/index.js";
  import CancelGuard from "#lib/run/CancelGuard.svelte";
  import DetailPane, { type ActionFeedback } from "#lib/run/DetailPane.svelte";
  import Map, { type MapSelection } from "#lib/run/Map.svelte";
  import SmallStates from "#lib/run/SmallStates.svelte";
  import Topbar from "#lib/run/Topbar.svelte";
  import Workspace from "#lib/run/Workspace.svelte";
  import {
    asMapSelection,
    currentActivitySelection,
    graphMatchesSnapshot,
    rebindSelection,
    runNavigationItems,
    type RunSelection,
  } from "#lib/run/selection.js";
  import { cancelRun, stopTurn } from "../../control.remote.js";
  import { answerInterview, type InterviewAnswer } from "../../interview.remote.js";
  import { steer, steerLoop, type LoopMessage, type Steer } from "../../steer.remote.js";
  import { invalidate } from "$app/navigation";
  import { tick, untrack } from "svelte";
  import { watchWindow } from "./window.remote.js";

  let { data }: { data: { snapshot: RunSnapshot; graph: string } } = $props();

  // SvelteKit may retain this page component while navigating between run IDs.
  // Replacing route data therefore replaces both sources of truth and causes
  // the SSE effect below to clean up the old connection before opening a new one.
  const observation = $derived(new RunObservation(data.snapshot));
  const graph = $derived<Graph | undefined>(
    data.graph ? (JSON.parse(data.graph) as Graph) : undefined,
  );
  const steerForm = steer.for("workspace-steer");
  const loopForm = steerLoop.for("workspace-loop");
  const answerForm = answerInterview.for("workspace-answer");

  let revision = $state(0);
  let selection = $state<RunSelection>();
  let reveal = $state<{ request: number; selection: MapSelection }>();
  let cancelOpen = $state(false);
  let stopping = $state(false);
  let cancelling = $state(false);
  let controlFeedback = $state("");
  let observedRunID = "";
  let revealSequence = 0;
  // Windowed live query (spike): when the connection has been down longer
  // than the window's own healing span (windowSpanMs), a still-open item
  // may have changed and dropped out of every yield the browser missed, so
  // the page resyncs from a fresh load rather than trusting the window to
  // catch it up on reconnect.
  let disconnectedSince: number | undefined;
  let resyncing = false;
  const windowSpanMs = 3000;
  // $state.raw, not $state: the live query instance manages its own runes
  // internally, and Svelte's deep-proxying of a plain $state wrapped around
  // a class with reactive accessors thrashed (observed while proving this
  // spike: the connected-tracking effect below re-ran thousands of times a
  // second with no state change). $state.raw only reacts when the reference
  // itself is reassigned, which is all a second effect needs.
  let liveInstance: ReturnType<typeof watchWindow> | undefined = $state.raw();
  let liveGeneration = 0;

  const snapshot = $derived.by(() => {
    revision;
    return observation.snapshot();
  });
  const connection = $derived.by((): ConnectionState => {
    revision;
    return observation.connection;
  });
  const graphMatches = $derived(graph ? graphMatchesSnapshot(graph, snapshot) : false);
  const searchItems = $derived(
    graph && graphMatches ? runNavigationItems(graph, snapshot) : [],
  );
  const currentSelection = $derived(
    graph && graphMatches ? currentActivitySelection(graph, snapshot) : undefined,
  );
  const activeTurn = $derived(
    Object.values(snapshot.turns)
      .filter((turn) => turn.ended === 0)
      .sort((left, right) => right.started - left.started)[0],
  );
  const waiting = $derived(
    Object.values(snapshot.interviews).filter((row) => row.status === "pending").length,
  );
  const openScopes = $derived(
    Object.values(snapshot.scopes).filter((scope) => scope.status === "running").length,
  );
  const activeSupervisors = $derived(
    Object.values(snapshot.turns).filter((turn) => {
      if (turn.ended) return false;
      const name = snapshot.sessions[turn.session]?.name ?? "";
      return name.includes("review") || name.includes("supervisor") || name.includes("critique");
    }).length,
  );

  const issueText = (issues: { message: string }[] | undefined, fallback: string) =>
    issues?.map((issue) => issue.message).join(" ") || fallback;

  $effect(() => {
    const next = observation;
    const runChanged = observedRunID !== "" && observedRunID !== next.run.id;
    const current = untrack(() => selection);
    revision = next.revision;
    selection = runChanged ? undefined : rebindSelection(current, next.snapshot());
    if (runChanged) {
      reveal = undefined;
      cancelOpen = false;
      controlFeedback = "";
    }
    observedRunID = next.run.id;
  });

  $effect(() => {
    const latest = snapshot;
    const current = untrack(() => selection);
    if (current) {
      const rebound = rebindSelection(current, latest);
      selection = rebound;
      if (!rebound) reveal = undefined;
    }
  });

  function navigateTo(next: RunSelection) {
    selection = next;
    const mapSelection = asMapSelection(next);
    if (mapSelection) reveal = { request: ++revealSequence, selection: mapSelection };
  }

  function selectWithoutReveal(next: RunSelection) {
    selection = next;
    reveal = undefined;
  }

  function clearSelection() {
    selection = undefined;
    reveal = undefined;
  }

  async function deliverSteer(request: Steer): Promise<ActionFeedback> {
    steerForm.fields.set(request);
    await tick();
    const submitted = await steerForm.submit();
    if (!submitted || !steerForm.result) {
      return {
        ok: false,
        message: issueText(steerForm.fields.allIssues(), "The message was not accepted."),
      };
    }
    return steerForm.result.landed
      ? { ok: true, message: "Sent into the running turn." }
      : { ok: false, message: "Not sent: the turn ended before it could receive the message." };
  }

  async function deliverLoop(request: LoopMessage): Promise<ActionFeedback> {
    loopForm.fields.set(request);
    await tick();
    const submitted = await loopForm.submit();
    if (!submitted || !loopForm.result) {
      return {
        ok: false,
        message: issueText(loopForm.fields.allIssues(), "The planner message was not accepted."),
      };
    }
    return {
      ok: true,
      message: request.wrap_up
        ? "Wrap-up is waiting for the planner’s next decision."
        : "Message is waiting for the planner’s next decision.",
    };
  }

  async function deliverAnswer(request: InterviewAnswer): Promise<ActionFeedback> {
    answerForm.fields.set(request);
    await tick();
    const submitted = await answerForm.submit();
    if (!submitted || !answerForm.result?.accepted) {
      return {
        ok: false,
        message: issueText(answerForm.fields.allIssues(), "The interview is no longer waiting."),
      };
    }
    return {
      ok: true,
      message: request.answer ? "Answer accepted by the waiting interview." : "Interview ended.",
    };
  }

  function selectScope(scopeKey: string) {
    const target = snapshot.scopes[scopeKey];
    if (target) selectWithoutReveal({ kind: "sheet", scope: target });
  }

  async function stopSelectedTurn(turn: TurnRow): Promise<ActionFeedback> {
    try {
      const result = await stopTurn({ run: snapshot.run.id, turn: turn.id });
      return result.accepted
        ? { ok: true, message: "Stop accepted. Waiting for the run record to update." }
        : { ok: false, message: "The turn did not accept the stop request." };
    } catch (error) {
      return { ok: false, message: error instanceof Error ? error.message : String(error) };
    }
  }

  async function stopActiveTurn() {
    if (!activeTurn || stopping) return;
    stopping = true;
    controlFeedback = "";
    try {
      const result = await stopTurn({ run: snapshot.run.id, turn: activeTurn.id });
      controlFeedback = result.accepted
        ? "Stop accepted. Waiting for the run record to update."
        : "The turn did not accept the stop request.";
      if (result.accepted) cancelOpen = false;
    } catch (error) {
      controlFeedback = error instanceof Error ? error.message : String(error);
    } finally {
      stopping = false;
    }
  }

  async function cancelActiveRun() {
    if (cancelling) return;
    cancelling = true;
    controlFeedback = "";
    try {
      const result = await cancelRun({ run: snapshot.run.id });
      controlFeedback = result.accepted
        ? "Cancellation accepted. Waiting for the run record to update."
        : "The run did not accept cancellation.";
      if (result.accepted) cancelOpen = false;
    } catch (error) {
      controlFeedback = error instanceof Error ? error.message : String(error);
    } finally {
      cancelling = false;
    }
  }

  // Windowed live query (spike; see ephemeral/research for the brief). One
  // `query.live` instance for the run replaces the hand-rolled EventSource
  // this effect used to open: every yield is UPSERTED into the observation
  // (applyWindow), which is idempotent, so a duplicate or a healed repeat
  // changes nothing. The effect below this one tracks the live instance's
  // own `connected` flag; once it has been down longer than the window's
  // own healing span, a change may have fallen out of every yield the
  // browser missed, so the page resyncs from a fresh load of this route
  // (not invalidateAll) instead of trusting the window to catch it up.
  $effect(() => {
    const generation = observation.beginConnection();
    if (observation.run.status !== "running") {
      liveInstance = undefined;
      return () => observation.endConnection(generation);
    }
    const instance = watchWindow({ run: observation.run.id });
    liveInstance = instance;
    liveGeneration = generation;
    let cancelled = false;

    void (async () => {
      try {
        for await (const win of instance) {
          if (cancelled || !observation.isCurrentConnection(generation)) return;
          if (observation.applyWindow(win)) revision++;
        }
      } catch {
        // Terminal failure. The connection effect below already reflects
        // this through instance.connected/instance.done and drives the
        // resync once it has lasted past the window's own healing span.
      }
    })();

    return () => {
      cancelled = true;
      if (liveInstance === instance) liveInstance = undefined;
      observation.endConnection(generation);
    };
  });

  // Tracks the live instance's own `connected` flag, kept at the top level
  // (not nested in the effect above) so it only reruns when connectivity
  // itself changes.
  //
  // Observed while proving this spike: this effect's body ran far more
  // often than `connected` actually changed value (a kit/Svelte
  // interaction this spike did not track down -- see the report). lastSeen
  // makes every action below idempotent against that: nothing happens
  // unless the value read this time differs from the value acted on last
  // time, which is what correctness here actually depends on.
  let lastSeenConnected: boolean | undefined;
  $effect(() => {
    const instance = liveInstance;
    if (!instance) return;
    const generation = liveGeneration;
    const connected = instance.connected;
    if (connected === lastSeenConnected) return;
    lastSeenConnected = connected;
    // eslint-disable-next-line no-console -- proof instrumentation for the spike; removed before merge.
    console.info("[window] connected=", connected);
    if (connected) {
      if (observation.connectionOpened(generation)) revision++;
      disconnectedSince = undefined;
      return;
    }
    if (disconnectedSince === undefined) disconnectedSince = Date.now();
    if (observation.connectionLost(generation)) revision++;
    if (!resyncing && Date.now() - disconnectedSince > windowSpanMs) {
      resyncing = true;
      // eslint-disable-next-line no-console -- proof instrumentation for the spike; harmless in prod.
      console.info("[window] resyncing after an outage longer than the healing span");
      void invalidate((url) => url.pathname === location.pathname).finally(() => {
        resyncing = false;
        disconnectedSince = undefined;
      });
    }
  });
</script>

<svelte:head><title>{snapshot.run.name} · Gimble</title></svelte:head>

<div class="run-workspace">
  <Topbar
    run={snapshot.run}
    {connection}
    {activeTurn}
    {waiting}
    {stopping}
    {cancelling}
    feedback={controlFeedback}
    {searchItems}
    currentAvailable={Boolean(currentSelection)}
    onnavigate={(item) => navigateTo(item.selection)}
    oncurrent={() => currentSelection && navigateTo(currentSelection)}
    onstop={stopActiveTurn}
    oncancel={() => (cancelOpen = true)}
  />
  <Workspace open={Boolean(selection)} onclose={clearSelection}>
    {#snippet map()}
      {#if graph && graphMatches}
        <Map
          {graph}
          {snapshot}
          selected={asMapSelection(selection)}
          {reveal}
          onselect={selectWithoutReveal}
        />
      {:else}
        <div class="graph-required">
          <SmallStates
            state="no-graph"
            workflowName={snapshot.run.name}
            graphProblem={graph ? "mismatch" : "missing"}
          />
        </div>
      {/if}
    {/snippet}
    {#snippet pane({ width, maximized, onmaximize })}
      <DetailPane
        {snapshot}
        {selection}
        {observation}
        {revision}
        {width}
        {maximized}
        {onmaximize}
        onsteer={deliverSteer}
        onloop={deliverLoop}
        onanswer={deliverAnswer}
        onstop={stopSelectedTurn}
        onselectscope={selectScope}
      />
    {/snippet}
  </Workspace>
</div>

<CancelGuard
  open={cancelOpen}
  run={snapshot.run}
  {activeTurn}
  {openScopes}
  supervisors={activeSupervisors}
  busy={stopping || cancelling}
  onopenchange={(open) => (cancelOpen = open)}
  onstop={stopActiveTurn}
  oncancel={cancelActiveRun}
/>

<form class="remote-form" {...steerForm} aria-hidden="true">
  <input {...steerForm.fields.run.as("hidden", steerForm.fields.run.value() ?? "")} />
  <input {...steerForm.fields.session.as("hidden", steerForm.fields.session.value() ?? "")} />
  <input {...steerForm.fields.message.as("hidden", steerForm.fields.message.value() ?? "")} />
</form>
<form class="remote-form" {...loopForm} aria-hidden="true">
  <input {...loopForm.fields.run.as("hidden", loopForm.fields.run.value() ?? "")} />
  <input {...loopForm.fields.scope.as("hidden", loopForm.fields.scope.value() ?? "")} />
  <input {...loopForm.fields.message.as("hidden", loopForm.fields.message.value() ?? "")} />
  <input {...loopForm.fields.wrap_up.as("hidden", loopForm.fields.wrap_up.value() ?? false)} />
</form>
<form class="remote-form" {...answerForm} aria-hidden="true">
  <input {...answerForm.fields.run.as("hidden", answerForm.fields.run.value() ?? "")} />
  <input
    {...answerForm.fields.question_id.as(
      "hidden",
      answerForm.fields.question_id.value() ?? "",
    )}
  />
  <input {...answerForm.fields.answer.as("hidden", answerForm.fields.answer.value() ?? "")} />
</form>

<style>
  :global(body:has(.run-workspace) .site-nav) {
    display: none;
  }

  :global(body:has(.run-workspace) main) {
    max-width: none;
    margin: 0;
    padding: 0;
  }

  .run-workspace {
    display: flex;
    height: 100vh;
    min-height: 560px;
    flex-direction: column;
    overflow: hidden;
    color: var(--foreground);
    font-family: var(--font-sans);
    background: var(--background);
  }

  .graph-required {
    box-sizing: border-box;
    min-width: 0;
    flex: 1;
    padding: 24px;
    overflow: auto;
    background: var(--background);
  }

  .remote-form {
    display: none;
  }
</style>
