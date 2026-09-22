<script lang="ts">
  import type { Graph } from "#lib/workflow/types.js";
  import {
    RunObservation,
    type ConnectionState,
    type ObservationDelta,
    type RunSnapshot,
    type TurnRow,
  } from "#lib/observation/index.js";
  import CancelGuard from "#lib/run/CancelGuard.svelte";
  import DetailPane, { type ActionFeedback } from "#lib/run/DetailPane.svelte";
  import Map, { type MapSelection } from "#lib/run/Map.svelte";
  import SessionPage from "#lib/run/detail/SessionPage.svelte";
  import SmallStates from "#lib/run/SmallStates.svelte";
  import Topbar from "#lib/run/Topbar.svelte";
  import Workspace from "#lib/run/Workspace.svelte";
  import {
    asMapSelection,
    currentActivitySelection,
    graphMatchesSnapshot,
    mapSelectionForTurn,
    rebindSelection,
    runNavigationItems,
    selectionSessionID,
    type RunSelection,
  } from "#lib/run/selection.js";
  import { watcherRows } from "#lib/run/watchers.js";
  import { cancelRun, stopTurn } from "../../../../control.remote.js";
  import { answerInterview, type InterviewAnswer } from "../../../../interview.remote.js";
  import { steer, steerLoop, type LoopMessage, type Steer } from "../../../../steer.remote.js";
	import { page } from "$app/state";
  import { tick, untrack } from "svelte";

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
  // The session shown on its own in place of the map and pane.
  let openSessionID = $state<string>();
  let observedRunID = "";
  let revealSequence = 0;

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

  const openSessionName = $derived(openSessionID ? snapshot.sessions[openSessionID]?.name : undefined);

  // The open session's agent call and its watchers, found from its latest
  // turn: the Source and Result tabs need them, as they do in the pane.
  const openSessionCall = $derived.by(() => {
    if (!openSessionID || !graph || !graphMatches) return undefined;
    const latest = Object.values(snapshot.turns)
      .filter((row) => row.session === openSessionID)
      .sort((left, right) => left.started - right.started)
      .at(-1);
    const found = latest ? mapSelectionForTurn(graph, snapshot, latest) : undefined;
    if (found?.kind !== "node" || found.operation.kind !== "agent_call") return undefined;
    return { definition: found.operation, watchers: watcherRows(snapshot, found.scope.key, found.operation) };
  });

  function openSession(next: RunSelection | undefined) {
    const id = selectionSessionID(next);
    if (id) openSessionID = id;
  }

  function closeSession() {
    openSessionID = undefined;
  }

  function isEditable(target: EventTarget | null) {
    const element = target as HTMLElement | null;
    return Boolean(element?.closest("input, textarea, [contenteditable=true]"));
  }

  function handleKeydown(event: KeyboardEvent) {
    if (cancelOpen || isEditable(event.target)) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === "Escape" && openSessionID) {
      event.preventDefault();
      closeSession();
    } else if (event.key === "o" && !openSessionID && selectionSessionID(selection)) {
      event.preventDefault();
      openSession(selection);
    }
  }

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
      openSessionID = undefined;
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

  $effect(() => {
    const generation = observation.beginConnection();
    let stream: EventSource | undefined;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;
    const delta = (message: MessageEvent<string>) => {
      if (!observation.isCurrentConnection(generation)) return;
      if (observation.applyDelta(JSON.parse(message.data) as ObservationDelta, generation)) {
        revision++;
      }
    };
    const replacement = (message: MessageEvent<string>) => {
      if (observation.replace(JSON.parse(message.data) as RunSnapshot, generation)) {
        revision++;
      }
    };
    const connect = () => {
      if (!observation.retryConnection(generation)) return;
      const currentAttempt = ++attempt;
      const query = new URLSearchParams({
        stream: observation.stream,
        position: String(observation.position),
      });
      const currentStream = new EventSource(
		`/projects/${page.params.project}/api/runs/${encodeURIComponent(observation.run.id)}/events?${query}`,
      );
      stream = currentStream;
      currentStream.addEventListener("delta", delta as EventListener);
      currentStream.addEventListener("snapshot", replacement as EventListener);
      currentStream.onopen = () => {
        if (attempt !== currentAttempt) return;
        if (observation.connectionOpened(generation)) revision++;
      };
      currentStream.onerror = () => {
        if (attempt !== currentAttempt) return;
        attempt++;
        const shouldRetry = observation.connectionLost(generation);
        if (observation.isCurrentConnection(generation)) revision++;
        currentStream.close();
        if (stream === currentStream) stream = undefined;
        if (shouldRetry) retry = setTimeout(connect, 250);
      };
    };
    if (observation.run.status === "running") connect();
    return () => {
      observation.endConnection(generation);
      if (retry !== undefined) clearTimeout(retry);
      stream?.close();
    };
  });
</script>

<svelte:head><title>{snapshot.run.name} · Gimble</title></svelte:head>

<svelte:window onkeydown={handleKeydown} />

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
    sessionName={openSessionName}
    onclosesession={closeSession}
  />
  <!-- Hidden, not unmounted, while a session is open, so the map keeps its
  scroll and expanded instances. -->
  <div class={["workspace-slot", { hidden: Boolean(openSessionID) }]}>
    <Workspace open={Boolean(selection) && !openSessionID} onclose={clearSelection}>
      {#snippet map()}
        {#if graph && graphMatches}
          <Map
            {graph}
            {snapshot}
            selected={asMapSelection(selection)}
            {reveal}
            onselect={selectWithoutReveal}
            onopen={openSession}
          />
        {:else}
          <div class="graph-required">
            <SmallStates
              state="no-graph"
			  backHref={`/projects/${page.params.project}`}
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
          onopen={() => openSession(selection)}
        />
      {/snippet}
    </Workspace>
  </div>
  {#if openSessionID}
    <div class="workspace-slot">
      <SessionPage
        {snapshot}
        {observation}
        {revision}
        sessionID={openSessionID}
        definition={openSessionCall?.definition}
        watchers={openSessionCall?.watchers}
        onsteer={deliverSteer}
        onstop={stopSelectedTurn}
        onscope={(scopeKey) => {
          closeSession();
          selectScope(scopeKey);
        }}
      />
    </div>
  {/if}
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

  .workspace-slot {
    display: flex;
    min-height: 0;
    flex: 1;
  }

  .workspace-slot.hidden {
    display: none;
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
