<script lang="ts">
  import InfoIcon from "@lucide/svelte/icons/info";
  import type {
    CommandRow,
    RunSnapshot,
    ScopeRow,
    TurnRow,
  } from "../observation/index.js";
  import Pip, { type PipState } from "./Pip.svelte";
  import type { HistorySelection, RunSelection } from "./selection.js";

  let {
    snapshot,
    reason = "No compiled graph matches this run",
    selected,
    onselect,
  }: {
    snapshot: RunSnapshot;
    reason?: string;
    selected?: RunSelection;
    onselect?: (selection: HistorySelection) => void;
  } = $props();

  type Lane =
    | { kind: "scope"; id: string; scope: ScopeRow; started: number; ended: number }
    | { kind: "turn"; id: string; scope: ScopeRow; turn: TurnRow; started: number; ended: number }
    | {
        kind: "command";
        id: string;
        scope: ScopeRow;
        command: CommandRow;
        started: number;
        ended: number;
      };

  const end = $derived.by(() => {
    if (snapshot.run.ended) return snapshot.run.ended;
    return Math.max(
      Date.now(),
      ...Object.values(snapshot.scopes).map((row) => row.ended || row.began),
      ...Object.values(snapshot.turns).map((row) => row.ended || row.started),
      ...Object.values(snapshot.commands).map((row) => row.ended || row.started),
    );
  });
  const span = $derived(Math.max(1, end - snapshot.run.started));
  const selectedID = $derived.by(() => {
    if (!selected) return "";
    if (selected.kind === "history-scope") return selected.scope.key;
    if (selected.kind === "history-turn") return selected.turn.id;
    if (selected.kind === "history-command") return selected.command.id;
    return "";
  });

  const lanes = $derived.by(() => {
    const rows: Lane[] = [];
    for (const scope of Object.values(snapshot.scopes)) {
      if (!scope.key) continue;
      rows.push({
        kind: "scope",
        id: scope.key,
        scope,
        started: scope.began,
        ended: scope.ended || end,
      });
    }
    for (const turn of Object.values(snapshot.turns)) {
      const scope = snapshot.scopes[turn.scope];
      if (scope) {
        rows.push({
          kind: "turn",
          id: turn.id,
          scope,
          turn,
          started: turn.started,
          ended: turn.ended || end,
        });
      }
    }
    for (const command of Object.values(snapshot.commands)) {
      const scope = snapshot.scopes[command.scope];
      if (scope) {
        rows.push({
          kind: "command",
          id: command.id,
          scope,
          command,
          started: command.started,
          ended: command.ended || end,
        });
      }
    }
    return rows.sort((left, right) => left.started - right.started || left.id.localeCompare(right.id));
  });

  const left = (lane: Lane) => `${((lane.started - snapshot.run.started) / span) * 100}%`;
  const width = (lane: Lane) => `${Math.max(0.4, ((lane.ended - lane.started) / span) * 100)}%`;
  const depth = (lane: Lane) => lane.scope.key.split("/").length - 1 + (lane.kind === "scope" ? 0 : 1);
  const label = (lane: Lane) => {
    if (lane.kind === "scope") return lane.scope.key;
    if (lane.kind === "command") return lane.command.name;
    const session = snapshot.sessions[lane.turn.session];
    return `${session?.name ?? lane.turn.session} / ${lane.turn.id.split("/").at(-1)}`;
  };
  const meta = (lane: Lane) => {
    if (lane.kind === "command") return lane.command.ended ? `exit ${lane.command.exit_code}` : "running";
    if (lane.kind === "turn") return lane.turn.ended ? "ended" : "running";
    return lane.scope.loop ? "loop" : "";
  };
  const state = (lane: Lane): PipState => {
    if (lane.kind === "command") {
      if (lane.command.interrupted) return "ended";
      if (lane.command.error || (lane.command.ended && lane.command.exit_code !== 0)) return "failed";
      return lane.command.ended ? "ended" : "running";
    }
    if (lane.kind === "turn") {
      if (lane.turn.interrupted) return "ended";
      if (lane.turn.error) return "failed";
      return lane.turn.ended ? "ended" : "running";
    }
    if (lane.scope.error) return "failed";
    return lane.scope.ended ? "ended" : "running";
  };
  const select = (lane: Lane) => {
    if (lane.kind === "scope") onselect?.({ kind: "history-scope", scope: lane.scope });
    else if (lane.kind === "turn")
      onselect?.({ kind: "history-turn", scope: lane.scope, turn: lane.turn });
    else onselect?.({ kind: "history-command", scope: lane.scope, command: lane.command });
  };
</script>

<section class="history" aria-label="Recorded run history">
  <div class="notice" role="status">
    <InfoIcon size={16} />
    <div>
      <strong>{reason}</strong>
      <p>
        The record below stands on its own: scopes, turns, commands, and their outcomes. No runtime
        fact is assigned to a call site the current graph cannot support.
      </p>
    </div>
  </div>
  <div class="description">
    <strong>Lanes</strong>
    <span>Scopes on the run’s clock. Overlap is concurrency; indentation is containment.</span>
  </div>
  <div class="lane-table">
    <div class="ruler">
      <strong>scope or activity</strong>
      <div class="ticks">
        {#each [0, 25, 50, 75, 100] as percent}
          <span class="tick" style:left={`${percent}%`}>
            {Math.round((span * percent) / 100 / 60_000)}m
          </span>
        {/each}
      </div>
    </div>
    <div class="rows">
      {#each lanes as lane (lane.id)}
        <button
          type="button"
          class="lane"
          class:selected={selectedID === lane.id}
          onclick={() => select(lane)}
        >
          <span class="lane-name" style:padding-left={`${8 + depth(lane) * 16}px`}>
            <Pip state={state(lane)} />
            <code>{label(lane)}</code>
            {#if meta(lane)}<small>{meta(lane)}</small>{/if}
          </span>
          <span class="lane-bars">
            <span
              class="bar"
              class:scope={lane.kind === "scope"}
              class:failed={state(lane) === "failed"}
              style:left={left(lane)}
              style:width={width(lane)}
            ></span>
          </span>
        </button>
      {/each}
    </div>
  </div>
</section>

<style>
  .history {
    display: flex;
    min-width: 0;
    flex: 1;
    flex-direction: column;
    gap: 14px;
    padding: 16px;
    overflow: auto;
    color: var(--foreground);
    background: var(--background);
  }

  .notice {
    display: flex;
    gap: 10px;
    padding: 12px 14px;
    font-size: 13px;
    line-height: 18px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .notice > :global(svg) {
    flex-shrink: 0;
    margin-top: 1px;
  }

  .notice p {
    margin: 3px 0 0;
    color: var(--status-muted);
  }

  .description {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 13px;
  }

  .description strong {
    padding: 5px 10px;
    border: 1px solid var(--border);
    border-radius: 6px;
  }

  .description span {
    color: var(--status-muted);
  }

  .lane-table {
    min-width: 700px;
    overflow: hidden;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: var(--radius);
  }

  .ruler,
  .lane {
    display: grid;
    grid-template-columns: 260px 1fr;
  }

  .ruler {
    height: 28px;
    align-items: center;
    border-bottom: 1px solid var(--border);
  }

  .ruler > strong {
    padding-left: 8px;
    font-size: 13px;
  }

  .ticks,
  .lane-bars {
    position: relative;
    height: 100%;
  }

  .tick {
    position: absolute;
    top: 0;
    bottom: 0;
    padding: 4px;
    color: var(--status-muted);
    font-family: var(--font-mono);
    font-size: 13px;
    border-left: 1px solid var(--border);
    transform: translateX(-1px);
  }

  .tick:last-child {
    transform: translateX(-100%);
  }

  .rows {
    padding: 6px 8px 8px;
  }

  .lane {
    width: 100%;
    height: 30px;
    align-items: center;
    padding: 0;
    color: var(--foreground);
    font: inherit;
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: 6px;
  }

  .lane:hover {
    background: var(--muted);
  }

  .lane.selected {
    background: var(--map-live-soft);
    box-shadow: inset 3px 0 0 var(--status-live);
  }

  .lane-name {
    display: flex;
    min-width: 0;
    box-sizing: border-box;
    align-items: center;
    gap: 8px;
  }

  .lane-name code {
    overflow: hidden;
    font-family: var(--font-mono);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .lane-name small {
    color: var(--status-muted);
    font-size: 13px;
    white-space: nowrap;
  }

  .bar {
    position: absolute;
    top: 9px;
    min-width: 3px;
    height: 12px;
    background: var(--foreground);
    border-radius: 3px;
  }

  .bar.scope {
    box-sizing: border-box;
    background: color-mix(in oklch, var(--foreground) 18%, transparent);
    border: 1px solid var(--map-line);
  }

  .bar.failed {
    background: var(--destructive);
    border-color: var(--destructive);
  }
</style>
