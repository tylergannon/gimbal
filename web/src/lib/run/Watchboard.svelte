<script lang="ts">
  import type { Graph } from "../workflow/types.js";
  import type { RunSnapshot, TurnRow } from "../observation/index.js";
  import WorkflowMap, { type MapSelection } from "./Map.svelte";
  import Pip from "./Pip.svelte";

  let {
    snapshot,
    graph,
    graphMatches,
    selectedSessionID,
    onfocus,
    onopenmap,
    onmapselect,
  }: {
    snapshot: RunSnapshot;
    graph?: Graph;
    graphMatches: boolean;
    selectedSessionID?: string;
    onfocus: (sessionID: string) => void;
    onopenmap: () => void;
    onmapselect: (selection: MapSelection) => void;
  } = $props();

  const activeTurns = $derived.by(() => {
    const latest = new Map<string, TurnRow>();
    for (const turn of Object.values(snapshot.turns)) {
      if (turn.ended !== 0 || !snapshot.sessions[turn.session]) continue;
      const previous = latest.get(turn.session);
      if (!previous || previous.started < turn.started) latest.set(turn.session, turn);
    }
    return [...latest.values()].sort((left, right) => left.started - right.started);
  });

  function updateFor(turn: TurnRow): { label: string; text: string } {
    const state = snapshot.transcripts[turn.id]?.snapshot.state;
    const messages = Object.values(state?.message ?? {}).flat();
    const assistant = messages.filter((message) => message.type === "assistant");
    const prose = assistant
      .flatMap((message) => message.content ?? [])
      .filter(
        (part) =>
          (part.type === "text" || part.type === "reasoning") &&
          typeof part.text === "string" &&
          part.text.trim(),
      )
      .at(-1);
    if (prose?.type === "text") return { label: "Recorded update", text: prose.text };
    if (prose?.type === "reasoning") return { label: "Recorded reasoning", text: prose.text };

    const tools = messages.flatMap((message) => message.content ?? []).filter((part) => part.type === "tool");
    return tools.length
      ? { label: "Recorded activity", text: `${tools.length} tool calls recorded` }
      : { label: "Recorded update", text: "No assistant prose recorded yet." };
  }
</script>

<section class="watchboard" aria-label="Watchboard">
  <div class="agents" aria-label="Running agents">
    {#if activeTurns.length}
      {#each activeTurns as turn (turn.session)}
        {@const session = snapshot.sessions[turn.session]}
        {@const update = updateFor(turn)}
        <button
          type="button"
          class="agent-card"
          class:selected={selectedSessionID === turn.session}
          aria-label={`Focus ${session.name}`}
          onclick={() => onfocus(turn.session)}
        >
          <span class="agent-head"><Pip state="running" /><strong>{session.name}</strong><span class="model">{session.model}</span></span>
          <span class="update-label">{update.label}</span>
          <span class="update">{update.text}</span>
          <span class="turn-meta">{turn.scope || "root"} · running</span>
        </button>
      {/each}
    {:else}
      <p class="empty">No agents have a running turn.</p>
    {/if}
    <section class="map-tile" aria-label="Workflow map tile">
      <button type="button" class="open-map" onclick={onopenmap}>Open map</button>
      {#if graph && graphMatches}
        <WorkflowMap
          {graph}
          {snapshot}
          selected={undefined}
          onselect={onmapselect}
          onopen={(selection) => {
            if (selection.kind !== "node" || selection.operation.kind !== "agent_call") return;
            const agentCall = selection.operation;
            const runtimeSessionID =
              selection.runtime && "session" in selection.runtime
                ? selection.runtime.session
                : undefined;
            const named = Object.values(snapshot.sessions).filter(
              (row) => row.name === agentCall.session,
            );
            const scoped = named.filter((row) => row.scope === selection.scope.key);
            const sessionID = runtimeSessionID ?? (scoped.length === 1 ? scoped[0].id : named.length === 1 ? named[0].id : undefined);
            if (sessionID && snapshot.sessions[sessionID]) onfocus(sessionID);
          }}
        />
      {:else}
        <div class="map-unavailable">Workflow map unavailable; running agents remain visible above.</div>
      {/if}
    </section>
  </div>
</section>

<style>
  .watchboard { display: flex; min-width: 0; min-height: 0; flex: 1; overflow: auto; padding: clamp(10px, 2vw, 20px); }
  .agents { display: grid; width: 100%; min-width: 0; align-content: start; align-items: start; grid-template-columns: repeat(auto-fit, minmax(min(100%, 280px), 1fr)); gap: 10px; }
  .agent-card { display: flex; min-width: 0; align-self: start; flex-direction: column; gap: 7px; padding: 10px 12px; color: var(--foreground); text-align: left; background: var(--card); border: 1px solid var(--map-line); border-radius: 9px; box-shadow: var(--shadow-xs); cursor: pointer; }
  .agent-card:hover, .agent-card:focus-visible { border-color: var(--status-live); outline-color: var(--status-live); }
  .agent-card.selected { border-color: var(--status-live); box-shadow: 0 0 0 2px var(--map-live-soft); }
  .agent-head { display: flex; min-width: 0; align-items: center; gap: 7px; }
  .agent-head strong { overflow: hidden; text-overflow: ellipsis; }
  .model { overflow: hidden; margin-left: auto; color: var(--status-muted); font: 11px var(--font-mono); text-overflow: ellipsis; white-space: nowrap; }
  .update-label { color: var(--status-muted); font-size: 10px; font-weight: 650; letter-spacing: .04em; text-transform: uppercase; }
  .update { display: -webkit-box; overflow: hidden; color: var(--foreground); font-size: 13px; line-height: 1.4; white-space: pre-wrap; overflow-wrap: anywhere; line-clamp: 2; -webkit-box-orient: vertical; -webkit-line-clamp: 2; }
  .turn-meta { color: var(--status-muted); font: 11px var(--font-mono); overflow-wrap: anywhere; }
  .empty, .map-unavailable { margin: 0; padding: 14px; color: var(--status-muted); font-size: 13px; }
  .map-tile { position: relative; min-width: 0; height: 260px; min-height: 220px; overflow: hidden; background: color-mix(in oklch, var(--muted) 35%, var(--background)); border: 1px solid var(--map-line); border-radius: 9px; }
  .map-tile :global(.map) { height: 100%; min-height: 0; }
  .open-map { position: absolute; z-index: 12; top: 8px; right: 8px; padding: 5px 9px; color: var(--foreground); font: inherit; font-size: 11px; background: var(--card); border: 1px solid var(--map-line); border-radius: 6px; box-shadow: var(--shadow-xs); cursor: pointer; }
  .map-tile :global(.map-controls) { top: 48px; left: 8px; }
  .open-map:hover, .open-map:focus-visible { border-color: var(--status-live); outline-color: var(--status-live); }
  @media (max-width: 600px) {
    .watchboard { padding: 8px; }
    .agents { grid-template-columns: minmax(0, 1fr); }
    .map-tile { height: 210px; min-height: 190px; }
  }
</style>
