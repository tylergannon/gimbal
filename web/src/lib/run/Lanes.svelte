<script module lang="ts">
  import type { CommandRow, RunSnapshot, ScopeRow, TurnRow } from "../observation/index.js";

  export type LaneSelection =
    | { kind: "scope"; row: ScopeRow }
    | { kind: "turn"; row: TurnRow }
    | { kind: "command"; row: CommandRow };

  type LaneRow = LaneSelection & {
    key: string;
    label: string;
    level: number;
    started: number;
    ended: number;
    status: "ended" | "running" | "failed";
    detail: string;
  };
</script>

<script lang="ts">
  import BotIcon from "@lucide/svelte/icons/bot";
  import BoxIcon from "@lucide/svelte/icons/box";
  import CircleAlertIcon from "@lucide/svelte/icons/circle-alert";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import Pip from "./Pip.svelte";

  let {
    snapshot,
    notice = "No compiled graph matches this run",
    onselect,
  }: {
    snapshot: RunSnapshot;
    notice?: string;
    onselect?: (selection: LaneSelection) => void;
  } = $props();

  let selectedKey = $state("");
  const commands = $derived(Object.values(snapshot.commands ?? {}));
  const rows = $derived.by(() => buildRows(snapshot, commands));
  const start = $derived(snapshot.run.started);
  const end = $derived(snapshot.run.ended || latestEnd(rows) || snapshot.run.started + 1);
  const span = $derived(Math.max(1, end - start));
  const ticks = $derived([0, 0.25, 0.5, 0.75, 1]);

  function parentKey(key: string) {
    const slash = key.lastIndexOf("/");
    return slash < 0 ? "" : key.slice(0, slash);
  }

  function buildRows(value: RunSnapshot, recordedCommands: CommandRow[]): LaneRow[] {
    const scopes = Object.values(value.scopes).filter((scope) => scope.key !== "");
    const turns = Object.values(value.turns);
    const children = new Map<string, ScopeRow[]>();
    for (const scope of scopes) {
      const parent = parentKey(scope.key);
      const bucket = children.get(parent) ?? [];
      bucket.push(scope);
      children.set(parent, bucket);
    }

    const failedScope = (scope: ScopeRow) =>
      Boolean(scope.error) ||
      recordedCommands.some(
        (command) =>
          (command.scope === scope.key || command.scope.startsWith(`${scope.key}/`)) &&
          (command.exit_code !== 0 || Boolean(command.error)),
      );

    const visit = (scope: ScopeRow, level: number): LaneRow[] => {
      const scopeRow: LaneRow = {
        kind: "scope",
        row: scope,
        key: `scope:${scope.key}`,
        label: scope.key.slice(scope.key.lastIndexOf("/") + 1),
        level,
        started: scope.began,
        ended: scope.ended || endOfScope(scope, turns, recordedCommands),
        status: failedScope(scope) ? "failed" : scope.ended ? "ended" : "running",
        detail: scope.loop ? "loop" : "scope",
      };
      const directScopes = children.get(scope.key) ?? [];
      const directTurns = turns.filter((turn) => turn.scope === scope.key);
      const directCommands = recordedCommands.filter((command) => command.scope === scope.key);
      const items = [
        ...directScopes.map((row) => ({ type: "scope" as const, started: row.began, row })),
        ...directTurns.map((row) => ({ type: "turn" as const, started: row.started, row })),
        ...directCommands.map((row) => ({ type: "command" as const, started: row.started, row })),
      ].sort((left, right) => left.started - right.started);

      return [
        scopeRow,
        ...items.flatMap((item): LaneRow[] => {
          if (item.type === "scope") return visit(item.row, level + 1);
          if (item.type === "turn") {
            const turn = item.row;
            return [
              {
                kind: "turn",
                row: turn,
                key: `turn:${turn.id}`,
                label: turn.id,
                level: level + 1,
                started: turn.started,
                ended: turn.ended || turn.started + turn.duration,
                status: turn.error ? "failed" : turn.ended ? "ended" : "running",
                detail: turn.interrupted ? "interrupted" : "agent call",
              },
            ];
          }
          const command = item.row;
          const failed = command.exit_code !== 0 || Boolean(command.error);
          return [
            {
              kind: "command",
              row: command,
              key: `command:${command.id}`,
              label: command.name,
              level: level + 1,
              started: command.started,
              ended: command.ended || command.started + command.duration,
              status: failed ? "failed" : command.ended ? "ended" : "running",
              detail: command.error || `exit ${command.exit_code}`,
            },
          ];
        }),
      ];
    };

    const topLevel = (children.get("") ?? []).sort((left, right) => left.began - right.began);
    return topLevel.flatMap((scope) => visit(scope, 0));
  }

  function endOfScope(scope: ScopeRow, turns: TurnRow[], recordedCommands: CommandRow[]) {
    return Math.max(
      scope.began,
      ...turns.filter((turn) => turn.scope === scope.key).map((turn) => turn.ended),
      ...recordedCommands.filter((command) => command.scope === scope.key).map((command) => command.ended),
    );
  }

  function latestEnd(laneRows: LaneRow[]) {
    return Math.max(0, ...laneRows.map((row) => row.ended));
  }

  function left(row: LaneRow) {
    return `${Math.max(0, ((row.started - start) / span) * 100)}%`;
  }

  function width(row: LaneRow) {
    return `${Math.max(0.7, ((Math.max(row.ended, row.started) - row.started) / span) * 100)}%`;
  }

  function elapsed(fraction: number) {
    const minutes = Math.round((span * fraction) / 60_000);
    return `${minutes}m`;
  }

  function select(row: LaneRow) {
    selectedKey = row.key;
    if (row.kind === "scope") onselect?.({ kind: "scope", row: row.row });
    else if (row.kind === "turn") onselect?.({ kind: "turn", row: row.row });
    else onselect?.({ kind: "command", row: row.row });
  }
</script>

<section class="lanes" aria-label="Recorded run lanes">
  <div class="notice" role="status">
    <CircleAlertIcon size={16} aria-hidden="true" />
    <div>
      <strong>{notice}</strong>
      <span>
        The current workflow changed after this run was recorded, so no map is drawn. The record
        below stands on its own: scopes, turns, commands and their outcomes.
      </span>
    </div>
  </div>

  <div class="lane-heading">
    <Badge variant="secondary">Lanes</Badge>
    <span>Scopes on the run’s clock. Overlap is concurrency; indentation is containment.</span>
  </div>

  <div class="clock">
    <div class="ruler lane-row">
      <div class="lane-name"><strong>scope</strong></div>
      <div class="lane-bars">
        {#each ticks as tick}
          <span class="tick" style:left={`${tick * 100}%`}></span>
          <span class:tick-end={tick === 1} class="tick-label" style:left={`${tick * 100}%`}>
            {elapsed(tick)}
          </span>
        {/each}
      </div>
    </div>

    <div class="lane-body">
      {#each rows as row (row.key)}
        <button
          type="button"
          class:selected={selectedKey === row.key}
          class="lane-row recorded-row"
          onclick={() => select(row)}
        >
          <span class="lane-name" style:padding-left={`${8 + row.level * 16}px`}>
            {#if row.kind === "scope"}
              <BoxIcon size={13} aria-hidden="true" />
            {:else if row.kind === "turn"}
              <BotIcon size={13} aria-hidden="true" />
            {:else}
              <TerminalIcon size={13} aria-hidden="true" />
            {/if}
            <Pip state={row.status} />
            <span class:command={row.kind === "command"} class="row-label">{row.label}</span>
            <span class="row-detail">{row.detail}</span>
          </span>
          <span class="lane-bars">
            <span
              class:scope-bar={row.kind === "scope"}
              class:failed-bar={row.status === "failed"}
              class:selected-bar={selectedKey === row.key}
              class="bar"
              style:left={left(row)}
              style:width={width(row)}
            ></span>
          </span>
        </button>
      {/each}
    </div>
  </div>

  <p class="footnote">
    Supervisor looks and steers are on each turn, not on this clock: open a turn to read them.
  </p>
</section>

<style>
  .lanes {
    display: flex;
    box-sizing: border-box;
    width: 100%;
    flex-direction: column;
    gap: 16px;
    padding: 16px;
    color: var(--foreground);
    background: var(--background);
  }

  .notice {
    display: grid;
    grid-template-columns: 16px 1fr;
    gap: 10px;
    padding: 12px 14px;
    font-size: 13px;
    line-height: 18px;
    background: var(--card);
    border: 1px solid var(--map-line);
    border-radius: var(--radius);
  }

  .notice div {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .notice span,
  .lane-heading span,
  .row-detail,
  .footnote {
    color: var(--status-muted);
  }

  .lane-heading {
    display: flex;
    align-items: center;
    gap: 12px;
    font-size: 13px;
  }

  .clock {
    overflow: hidden;
    background: var(--card);
    border: 1px solid var(--map-line);
    border-radius: var(--radius);
  }

  .lane-row {
    display: flex;
    box-sizing: border-box;
    width: 100%;
    height: 30px;
    align-items: center;
  }

  .ruler {
    height: 26px;
    border-bottom: 1px solid var(--border);
  }

  .lane-name {
    display: flex;
    box-sizing: border-box;
    width: 250px;
    flex: 0 0 250px;
    align-items: center;
    gap: 7px;
    padding: 0 8px;
    overflow: hidden;
    font-size: 13px;
    line-height: 18px;
    white-space: nowrap;
  }

  .lane-bars {
    position: relative;
    height: 100%;
    flex: 1;
    min-width: 0;
  }

  .tick {
    position: absolute;
    top: 0;
    bottom: 0;
    border-left: 1px solid var(--border);
  }

  .tick-label {
    position: absolute;
    top: 3px;
    padding-left: 4px;
    color: var(--status-muted);
    font-family: var(--font-mono);
    font-size: 13px;
    line-height: 18px;
    transform: translateX(0);
  }

  .tick-end {
    padding-right: 6px;
    padding-left: 0;
    color: var(--destructive);
    font-weight: 600;
    transform: translateX(-100%);
  }

  .lane-body {
    display: flex;
    flex-direction: column;
    gap: 1px;
    padding: 6px 8px 8px;
  }

  .recorded-row {
    padding: 0;
    color: var(--foreground);
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: calc(var(--radius) - 4px);
  }

  .recorded-row:hover,
  .recorded-row:focus-visible,
  .recorded-row.selected {
    background: var(--map-live-soft);
    outline: none;
  }

  .recorded-row.selected {
    box-shadow: inset 3px 0 0 var(--status-live);
  }

  .row-label {
    overflow: hidden;
    font-family: var(--font-mono);
    text-overflow: ellipsis;
  }

  .row-label.command {
    font-weight: 500;
  }

  .row-detail {
    flex-shrink: 0;
    font-size: 13px;
  }

  .bar {
    position: absolute;
    top: 9px;
    height: 12px;
    min-width: 3px;
    background: var(--foreground);
    border-radius: 3px;
  }

  .scope-bar {
    box-sizing: border-box;
    background: color-mix(in oklch, var(--foreground) 18%, transparent);
    border: 1px solid var(--map-line);
  }

  .failed-bar {
    background: var(--destructive);
    border-color: var(--destructive);
  }

  .selected-bar {
    outline: 2px solid var(--status-live);
    outline-offset: 2px;
  }

  .footnote {
    margin: 0;
    font-size: 13px;
    line-height: 18px;
  }

  @media (max-width: 760px) {
    .lane-name {
      width: 210px;
      flex-basis: 210px;
    }
  }
</style>
