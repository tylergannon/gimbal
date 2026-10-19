<script lang="ts">
  import { afterNavigate } from "$app/navigation";
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { currentUser } from "../../routes/auth.remote";
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
  import type { MapSelection } from "#lib/run/types.js";
  import RunMap from "#lib/run/RunMap.svelte";
  import { sourceFromGraph } from "#lib/run/runSource.js";
  import Inspector from "#lib/workflow-design/Inspector.svelte";
  import type { SourcePage, ViewNode } from "#lib/workflow-design/types.js";
  import SessionPage from "#lib/run/detail/SessionPage.svelte";
  import RunQuestions from "./RunQuestions.svelte";
  import SmallStates from "#lib/run/SmallStates.svelte";
  import Topbar from "./Topbar.svelte";
  import RunHeader from "./RunHeader.svelte";
  import RunTranscript from "./RunTranscript.svelte";
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
  import { cancelRun, stopTurn } from "../../routes/control.remote.js";
  import { answerInterview, type InterviewAnswer } from "../../routes/interview.remote.js";
  import { steerLoop, type LoopMessage } from "../../routes/steer.remote.js";
	import { page } from "$app/state";

  import { tick, untrack } from "svelte";

  let { project, data, active = true, onstate, onrefresh }: { project: string; data: { snapshot: RunSnapshot; graph: string; source?: string }; active?: boolean; onstate?: (run: RunSnapshot["run"], waiting: number) => void; onrefresh?: () => Promise<unknown> } = $props();

  // SvelteKit may retain this page component while navigating between run IDs.
  // Replacing route data therefore replaces both sources of truth and causes
  // the SSE effect below to clean up the old connection before opening a new one.
  const observation = $derived(new RunObservation(data.snapshot));
  const graph = $derived<Graph | undefined>(
    data.graph ? (JSON.parse(data.graph) as Graph) : undefined,
  );
  // The designer's page for this workflow, saved by the run. Runs saved
  // before that draw from their graph alone.
  const source = $derived<SourcePage | undefined>(
    data.source ? (JSON.parse(data.source) as SourcePage) : graph ? sourceFromGraph(graph) : undefined,
  );
  const loopForm = $derived(steerLoop.for(`workspace-loop-${project}-${data.snapshot.run.id}`));
  const answerForm = $derived(answerInterview.for(`workspace-answer-${project}-${data.snapshot.run.id}`));

  let revision = $state(0);
  let selection = $state<RunSelection>();
  /** A step with no run record, shown in the designer's inspector. */
  let inspected = $state<ViewNode>();
  let inspectorTypes = $state(false);
  let inspectorPaths = $state<string[]>([]);
  let reveal = $state<{ request: number; selection: MapSelection }>();
  let cancelOpen = $state(false);
  let stopping = $state(false);
  let cancelling = $state(false);
  let controlFeedback = $state("");
  type RunView = "transcript" | "map" | "focus";
  let view = $state<RunView>("transcript");
  let selectedSessionID = $state<string | undefined>();
  let mapOpened = $state(false);
  function showMap() { mapOpened = true; view = "map"; }
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
    snapshot.run.status === "running" ? Object.values(snapshot.interviews).filter((row) => row.status === "pending").length : 0,
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

  const openSessionName = $derived(view === "focus" && selectedSessionID ? snapshot.sessions[selectedSessionID]?.name : undefined);

  // The open session's agent call and its watchers, found from its latest
  // turn: the Source and Result tabs need them, as they do in the pane.
  const openSessionCall = $derived.by(() => {
    if (!selectedSessionID || !graph || !graphMatches) return undefined;
    const latest = Object.values(snapshot.turns)
      .filter((row) => row.session === selectedSessionID)
      .sort((left, right) => left.started - right.started)
      .at(-1);
    const found = latest ? mapSelectionForTurn(graph, snapshot, latest) : undefined;
    if (found?.kind !== "node" || found.operation.kind !== "agent_call") return undefined;
    return { definition: found.operation, watchers: watcherRows(snapshot, found.scope.key, found.operation) };
  });

  function openSession(next: RunSelection | undefined) {
    const id = selectionSessionID(next);
    if (id) {
      selectedSessionID = id;
      view = "focus";
      cancelOpen = false;
    }
  }

  function closeSession() {
    view = "map";
  }

  function isEditable(target: EventTarget | null) {
    const element = target as HTMLElement | null;
    return Boolean(element?.closest("input, textarea, [contenteditable=true]"));
  }

  function handleKeydown(event: KeyboardEvent) {
    if (!active || cancelOpen || isEditable(event.target)) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    if (event.key === "Escape" && view === "focus") {
      event.preventDefault();
      closeSession();
    } else if (event.key === "o" && view !== "focus" && selectionSessionID(selection)) {
      event.preventDefault();
      openSession(selection);
    }
  }

  const issueText = (issues: { message: string }[] | undefined, fallback: string) =>
    issues?.map((issue) => issue.message).join(" ") || fallback;

  $effect(() => { const run = snapshot.run; const count = waiting; untrack(() => onstate?.(run, count)); });

  afterNavigate(() => { cancelOpen = false; });

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
      inspected = undefined;
      view = "transcript";
      mapOpened = false;
      selectedSessionID = undefined;
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
    inspected = undefined;
    const mapSelection = asMapSelection(next);
    if (mapSelection) reveal = { request: ++revealSequence, selection: mapSelection };
  }

  function selectWithoutReveal(next: RunSelection) {
    selection = next;
    inspected = undefined;
    reveal = undefined;
  }

  function selectMapItem(next: MapSelection) {
    selectWithoutReveal(next);
    if (next.kind === "node" && next.operation.kind === "agent_call") {
      openSession(next);
    } else {
      view = "map";
    }
  }

  function inspect(node: ViewNode) {
    selection = undefined;
    reveal = undefined;
    inspected = node;
    inspectorPaths = [];
  }

  function clearSelection() {
    selection = undefined;
    inspected = undefined;
    reveal = undefined;
  }

  async function deliverLoop(request: LoopMessage): Promise<ActionFeedback> {
    try {
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
    } catch (error) {
      navigateOnUnauthorized(error);
      return { ok: false, message: error instanceof Error ? error.message : String(error) };
    }
  }

  async function deliverAnswer(request: InterviewAnswer): Promise<ActionFeedback> {
    try {
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
    } catch (error) {
      navigateOnUnauthorized(error);
      return { ok: false, message: error instanceof Error ? error.message : String(error) };
    }
  }

  function selectScope(scopeKey: string) {
    const target = snapshot.scopes[scopeKey];
    if (target) selectWithoutReveal({ kind: "sheet", scope: target });
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
      navigateOnUnauthorized(error);
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
      if (result.accepted) {cancelOpen = false;}
    } catch (error) {
      navigateOnUnauthorized(error);
      controlFeedback = "The cancellation request could not be confirmed. Check the run's current status.";
      cancelOpen = false;
    } finally {
      cancelling = false;
    }
  }

  $effect(() => {
    if (!active) return;
    const authEnabled = page.data.authEnabled;
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
		`/projects/${project}/api/runs/${encodeURIComponent(observation.run.id)}/events?${query}`,
      );
      stream = currentStream;
      currentStream.addEventListener("delta", delta as EventListener);
      currentStream.addEventListener("snapshot", replacement as EventListener);
      currentStream.onopen = () => {
        if (attempt !== currentAttempt) return;
        if (observation.connectionOpened(generation)) revision++;
      };
      currentStream.onerror = async () => {
        if (attempt !== currentAttempt) return;
        const failedAttempt = ++attempt;
        const shouldRetry = observation.connectionLost(generation);
        if (observation.isCurrentConnection(generation)) revision++;
        currentStream.close();
        if (stream === currentStream) stream = undefined;
        if (!shouldRetry) return;
        if (authEnabled) {
          try {
            const user = currentUser();
            await user.refresh();
            await user;
          } catch (error) {
            if (!observation.isCurrentConnection(generation) || attempt !== failedAttempt) return;
            if (navigateOnUnauthorized(error)) return;
          }
        }
        if (observation.retryConnection(generation) && attempt === failedAttempt)
          retry = setTimeout(connect, 250);
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


<svelte:window onkeydown={handleKeydown} />

<div class="run-workspace">
  {#if view === 'transcript'}
    <RunHeader run={snapshot.run} {waiting} {connection} mode={view} onmode={(mode) => mode === "map" ? showMap() : (view = mode)} oncancel={() => snapshot.run.status === "running" ? (cancelOpen = true) : void cancelActiveRun()} {searchItems} onnavigate={(item) => { showMap(); navigateTo(item.selection); }} />
    {#if controlFeedback}<p class="placement" role="status">{controlFeedback}</p>{/if}
  {:else}
    <div class="map-toolbar"><button class="transcript-back" onclick={() => view = 'transcript'}>← Transcript</button></div>
    <Topbar {project} {active} run={snapshot.run} agentFocus={view === "focus"} {connection} {activeTurn} {waiting} {stopping} {cancelling} feedback={controlFeedback} {searchItems} currentAvailable={Boolean(currentSelection)} onnavigate={(item) => navigateTo(item.selection)} oncurrent={() => currentSelection && navigateTo(currentSelection)} onstop={stopActiveTurn} oncancel={() => snapshot.run.status === "running" ? (cancelOpen = true) : void cancelActiveRun()} sessionName={openSessionName} onclosesession={closeSession} />
    {#if waiting}<div class="run-questions" aria-label="Run questions">{#each Object.values(snapshot.interviews).filter(question => question.status === 'pending') as question (question.question_id)}<RunQuestions run={snapshot.run.id} {question} />{/each}</div>{/if}
  {/if}
  <div class={["workspace-slot", { hidden: view !== "transcript" }]}>
    <RunTranscript {project} {snapshot} {observation} {revision} graph={graphMatches ? graph : undefined} onmap={(next) => { showMap(); if (next) selectWithoutReveal(next); }} />
  </div>
  <!-- Hidden, not unmounted, while a session is open, so the map keeps its
  scroll and expanded instances. -->
  {#if mapOpened}
  <div class={["workspace-slot", { hidden: view !== "map" }]}>
    <Workspace active={active && view === "map"} open={Boolean(selection || inspected)} onclose={clearSelection}>
      {#snippet map()}
        {#if graph && source && graphMatches}
          <RunMap
            {source}
            {graph}
            {snapshot}
            selected={asMapSelection(selection)}
            {inspected}
            {reveal}
            onselect={selectMapItem}
            oninspect={inspect}
            onopen={openSession}
          />
        {:else}
          <div class="graph-required">
            <SmallStates
              state="no-graph"
			  backHref={`/projects/${project}`}
              workflowName={snapshot.run.name}
              graphProblem={graph ? "mismatch" : "missing"}
            />
          </div>
        {/if}
      {/snippet}
      {#snippet pane({ width, maximized, onmaximize })}
        {#if inspected && !selection}
          <div class="static-inspector" style:width={`${width}px`}>
            <Inspector
              node={inspected}
              onclose={clearSelection}
              goTypes={inspectorTypes}
              ontypes={(types) => (inspectorTypes = types)}
              openPaths={inspectorPaths}
              ondetail={(path, open) =>
                (inspectorPaths = open ? [...inspectorPaths, path] : inspectorPaths.filter((p) => p !== path))}
            />
          </div>
        {:else}
        <DetailPane
          {snapshot}
          {selection}
          {observation}
          {revision}
          {width}
          {maximized}
          {onmaximize}
          onloop={deliverLoop}
          onanswer={deliverAnswer}
          onselectscope={selectScope}
          onopen={() => openSession(selection)}
        />
        {/if}
      {/snippet}
    </Workspace>
  </div>
  {/if}
  {#if view === "focus" && selectedSessionID}
    <div class="workspace-slot">
      {#key selectedSessionID}
      <SessionPage
        {project}
        {snapshot}
        {observation}
        {revision}
        sessionID={selectedSessionID}
        definition={openSessionCall?.definition}
        watchers={openSessionCall?.watchers}
        onscope={(scopeKey) => {
          closeSession();
          selectScope(scopeKey);
        }}
      />
      {/key}
    </div>
  {/if}
</div>

<CancelGuard
  open={active && cancelOpen}
  run={snapshot.run}
  {activeTurn}
  {openScopes}
  supervisors={activeSupervisors}
  busy={stopping || cancelling}
  onopenchange={(open) => (cancelOpen = open)}
  onstop={stopActiveTurn}
  oncancel={cancelActiveRun}
/>

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
  .map-toolbar { padding: 6px 16px 0; } .transcript-back { border: 0; color: var(--muted-foreground); background: transparent; font: 12px var(--font-sans); cursor: pointer; }
  .run-questions { flex: none; max-height: 30vh; overflow: auto; }
 .placement{margin:0;padding:6px 20px;font-size:13px;color:#374151;background:#f7f8fa}
  .run-workspace {
    display: flex;
    height: 100%;
    min-height: 0;
    flex-direction: column;
    overflow: hidden;
    color: var(--foreground);
    font-family: var(--font-sans);
    background: var(--background);
  }

  .workspace-slot {
    display: flex;
    min-height: 0;
    min-width: 0;
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

  .static-inspector { height: 100%; min-height: 0; overflow: auto; background: var(--background); }

  .remote-form {
    display: none;
  }

</style>
