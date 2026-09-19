<script module lang="ts">
  export type ConnectionStatus = "live" | "disconnected";
</script>

<script lang="ts">
  import SearchIcon from "@lucide/svelte/icons/search";
  import SquareIcon from "@lucide/svelte/icons/square";
  import XIcon from "@lucide/svelte/icons/x";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  import type { RunRow } from "../observation/index.js";

  let {
    run,
    connection = "live",
    elapsed,
    current = "coding · task 3",
    stopDisabled = false,
    onsearch,
    oncurrentselect,
    onstopturn,
    oncancelrun,
  }: {
    run: RunRow;
    connection?: ConnectionStatus;
    elapsed: string;
    current?: string;
    stopDisabled?: boolean;
    onsearch?: (query: string) => void;
    oncurrentselect?: () => void;
    onstopturn?: () => void;
    oncancelrun?: () => void;
  } = $props();

  let query = $state("");
  const statusLabel = $derived(run.status[0].toUpperCase() + run.status.slice(1));
</script>

<header class="topbar">
  <nav aria-label="Breadcrumb">
    <ol>
      <li><a href="/runs">Runs</a></li>
      <li class="separator" aria-hidden="true">/</li>
      <li class="workflow">{run.name}</li>
      <li class="separator" aria-hidden="true">/</li>
      <li class="run-id">{run.id}</li>
    </ol>
  </nav>

  <div class="status-group">
    <Badge variant="outline" class="status-badge">
      <span class="status-mark" data-status={run.status}></span>
      {statusLabel}
    </Badge>
    <Badge variant="secondary" class="status-badge">
      <span class="connection-mark" data-connection={connection}></span>
      {connection === "live" ? "Live" : "Disconnected"}
    </Badge>
    <span class="elapsed">{elapsed}</span>
  </div>

  <button type="button" class="current" onclick={() => oncurrentselect?.()}>
    <span>now</span>
    <strong>{current}</strong>
  </button>

  <div class="spacer"></div>

  <label class="search">
    <SearchIcon size={14} aria-hidden="true" />
    <Input
      aria-label="Find a scope or turn"
      placeholder="Find a scope or turn"
      bind:value={query}
      oninput={() => onsearch?.(query)}
    />
    <kbd>/</kbd>
  </label>

  <div class="actions">
    <Button
      variant="outline"
      size="sm"
      disabled={stopDisabled}
      onclick={() => onstopturn?.()}
    >
      <SquareIcon data-icon="inline-start" />
      Stop turn
    </Button>
    <Button variant="outline" size="sm" class="cancel" onclick={() => oncancelrun?.()}>
      <XIcon data-icon="inline-start" />
      Cancel run
    </Button>
  </div>
</header>

<style>
  .topbar {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    width: 100%;
    height: 56px;
    gap: 16px;
    padding: 0 16px;
    color: var(--foreground);
    background: var(--card);
    border-bottom: 1px solid var(--map-line);
  }

  nav {
    min-width: 0;
  }

  ol,
  .status-group,
  .actions {
    display: flex;
    align-items: center;
  }

  ol {
    gap: 7px;
    padding: 0;
    margin: 0;
    list-style: none;
    white-space: nowrap;
  }

  a,
  .current strong {
    color: var(--status-live);
    text-decoration: none;
  }

  .workflow {
    overflow: hidden;
    max-width: 190px;
    font-size: 15px;
    font-weight: 600;
    text-overflow: ellipsis;
  }

  .separator,
  .elapsed,
  .current span {
    color: var(--status-muted);
  }

  .run-id,
  .elapsed {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .status-group {
    flex-shrink: 0;
    gap: 8px;
  }

  :global(.status-badge) {
    gap: 6px;
  }

  .status-mark,
  .connection-mark {
    display: inline-block;
    box-sizing: border-box;
    width: 8px;
    height: 8px;
    flex-shrink: 0;
    border-radius: 999px;
  }

  .status-mark[data-status="running"] {
    border: 2px solid var(--status-running);
    border-top-color: transparent;
    animation: spin 1s linear infinite;
  }

  .status-mark[data-status="completed"],
  .connection-mark[data-connection="live"] {
    background: var(--foreground);
  }

  .status-mark[data-status="failed"] {
    position: relative;
    background: var(--destructive);
  }

  .status-mark[data-status="failed"]::before,
  .status-mark[data-status="failed"]::after {
    position: absolute;
    top: 3.25px;
    right: 1.5px;
    left: 1.5px;
    height: 1.5px;
    content: "";
    background: var(--background);
    transform: rotate(45deg);
  }

  .status-mark[data-status="failed"]::after {
    transform: rotate(-45deg);
  }

  .status-mark[data-status="cancelled"] {
    background: var(--muted-foreground);
  }

  .connection-mark[data-connection="disconnected"] {
    border: 1.5px dashed var(--status-muted);
  }

  .current {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 0 0 0 8px;
    font: inherit;
    font-size: 13px;
    cursor: pointer;
    background: transparent;
    border: 0;
    border-left: 1px solid var(--map-line-soft);
  }

  .current strong {
    font-weight: 500;
  }

  .spacer {
    flex: 1;
  }

  .search {
    display: flex;
    align-items: center;
    width: 240px;
    height: 32px;
    padding: 0 9px;
    color: var(--muted-foreground);
    background: var(--background);
    border: 1px solid var(--map-line);
    border-radius: calc(var(--radius) - 2px);
  }

  .search :global(input) {
    height: 30px;
    padding: 0 6px;
    font-size: 13px;
    box-shadow: none;
    border: 0;
  }

  .search :global(input:focus-visible) {
    box-shadow: none;
  }

  kbd {
    display: inline-flex;
    min-width: 18px;
    height: 18px;
    align-items: center;
    justify-content: center;
    font-family: var(--font-mono);
    font-size: 11px;
    background: var(--muted);
    border: 1px solid var(--border);
    border-radius: 4px;
  }

  .actions {
    gap: 8px;
  }

  :global(.cancel) {
    color: var(--destructive);
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (max-width: 1050px) {
    .current,
    .run-id,
    .separator:last-of-type {
      display: none;
    }

    .search {
      width: 200px;
    }
  }
</style>
