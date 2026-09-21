<script lang="ts">
  import SearchIcon from "@lucide/svelte/icons/search";
  import StopIcon from "@lucide/svelte/icons/square";
  import XIcon from "@lucide/svelte/icons/x";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  import type { ConnectionState, RunRow, TurnRow } from "../observation/index.svelte.js";
  import Pip from "./Pip.svelte";
  import type { RunNavigationItem } from "./selection.js";

  let {
    run,
    connection,
    activeTurn,
    waiting = 0,
    stopping = false,
    cancelling = false,
    feedback = "",
    searchItems = [],
    currentAvailable = false,
    onnavigate,
    oncurrent,
    onstop,
    oncancel,
  }: {
    run: RunRow;
    connection: ConnectionState;
    activeTurn?: TurnRow;
    waiting?: number;
    stopping?: boolean;
    cancelling?: boolean;
    feedback?: string;
    searchItems?: RunNavigationItem[];
    currentAvailable?: boolean;
    onnavigate?: (item: RunNavigationItem) => void;
    oncurrent?: () => void;
    onstop?: () => void;
    oncancel?: () => void;
  } = $props();

  let now = $state(Date.now());
  let query = $state("");
  let searchOpen = $state(false);
  let searchInput = $state<HTMLInputElement | null>(null);
  const terminal = $derived(run.status !== "running");
  const elapsed = $derived(formatDuration((run.ended || now) - run.started));
  const statusVariant = $derived(run.status === "failed" ? "destructive" : "outline");

  const disconnectedAt = $derived(connection === "disconnected" ? Date.now() : 0);

  $effect(() => {
    if (terminal && connection !== "disconnected") return;
    now = Date.now();
    const timer = window.setInterval(() => (now = Date.now()), 1_000);
    return () => window.clearInterval(timer);
  });

  const disconnectedSeconds = $derived(
    disconnectedAt ? Math.max(0, Math.floor((now - disconnectedAt) / 1_000)) : 0,
  );

  function formatDuration(milliseconds: number) {
    const seconds = Math.max(0, Math.floor(milliseconds / 1000));
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const rest = seconds % 60;
    return hours > 0
      ? `${hours}h ${minutes.toString().padStart(2, "0")}m`
      : `${minutes}m ${rest.toString().padStart(2, "0")}s`;
  }

  const currentLabel = $derived.by(() => {
    if (waiting > 0) return `${waiting} question${waiting === 1 ? "" : "s"} waiting`;
    if (!activeTurn) return terminal ? "recorded run" : "waiting for activity";
    const scope = activeTurn.scope.split("/").at(-1)?.replace(".", " ") ?? "run";
    return `now ${scope}`;
  });

  const matches = $derived.by(() => {
    const needle = query.trim().toLocaleLowerCase();
    if (!needle) return [];
    const terms = needle.split(/\s+/);
    return searchItems
      .filter((item) => {
        const text = `${item.label} ${item.context}`.toLocaleLowerCase();
        return terms.every((term) => text.includes(term));
      })
      .slice(0, 8);
  });

  function choose(item: RunNavigationItem) {
    query = item.label;
    searchOpen = false;
    onnavigate?.(item);
  }

  function searchKeydown(event: KeyboardEvent) {
    if (event.key === "Escape") {
      searchOpen = false;
      searchInput?.blur();
    } else if (event.key === "Enter" && matches[0]) {
      event.preventDefault();
      choose(matches[0]);
    }
  }

  function shortcut(event: KeyboardEvent) {
    if (event.key !== "/" || event.metaKey || event.ctrlKey || event.altKey) return;
    const target = event.target as HTMLElement;
    if (target.closest("input, textarea, [contenteditable=true]")) return;
    event.preventDefault();
    searchInput?.focus();
    searchOpen = true;
  }
</script>

<svelte:window onkeydown={shortcut} />

<header class="topbar">
  <nav aria-label="Breadcrumb" class="breadcrumb">
    <a href="/">Runs</a><span aria-hidden="true">›</span><strong>{run.name}</strong
    ><span aria-hidden="true">›</span><code>{run.id.split(".")[0]?.slice(-8) || run.id}</code>
  </nav>

  <div class="states">
    <Badge variant={statusVariant} class="status">
      {#if run.status === "running"}<Pip state="running" />{/if}
      {run.status[0].toUpperCase() + run.status.slice(1)}
    </Badge>
    {#if !terminal}
      {#if connection === "disconnected"}
        <Badge variant="outline" class="disconnected">
          <Pip state="not-yet" /> Disconnected {disconnectedSeconds} s
        </Badge>
      {:else if connection === "live"}
        <Badge variant="secondary">
          <Pip state="ended" /> Live
        </Badge>
      {:else}
        <Badge variant="secondary"><Pip state="running" /> Connecting</Badge>
      {/if}
    {/if}
    <span class="elapsed">{elapsed}</span>
  </div>

  <button
    type="button"
    class="current"
    class:waiting={waiting > 0}
    disabled={!currentAvailable}
    title={currentAvailable ? "Reveal current activity" : currentLabel}
    onclick={oncurrent}
  >
    {#if waiting > 0}<Pip state="waiting" />{/if}
    {currentLabel}
  </button>

  <div class="spacer"></div>
  <label class="search">
    <SearchIcon size={14} />
    <Input
      bind:ref={searchInput}
      bind:value={query}
      aria-label="Find a scope, turn, or command"
      aria-expanded={searchOpen && matches.length > 0}
      aria-controls="run-search-results"
      placeholder="Find a scope or turn"
      onfocus={() => (searchOpen = true)}
      oninput={() => (searchOpen = true)}
      onkeydown={searchKeydown}
      onblur={() => window.setTimeout(() => (searchOpen = false), 100)}
    />
    <kbd>/</kbd>
    {#if searchOpen && query.trim()}
      <div id="run-search-results" class="search-results" role="listbox">
        {#each matches as item (item.id)}
          <button type="button" role="option" aria-selected="false" onclick={() => choose(item)}>
            <strong>{item.label}</strong><span>{item.context}</span>
          </button>
        {:else}
          <p>No matching scope, turn, or command</p>
        {/each}
      </div>
    {/if}
  </label>
  {#if !terminal}
    <div class="controls">
      <Button
        variant="outline"
        size="sm"
        disabled={!activeTurn || stopping}
        title={activeTurn ? `Stop ${activeTurn.id}` : "No turn is running"}
        onclick={onstop}
      >
        <StopIcon data-icon="inline-start" size={16} />{stopping ? "Stopping" : "Stop turn"}
      </Button>
      <Button
        variant="outline"
        size="sm"
        class="cancel"
        disabled={cancelling}
        onclick={oncancel}
      >
        <XIcon data-icon="inline-start" size={16} />{cancelling ? "Cancelling" : "Cancel run"}
      </Button>
    </div>
  {:else}
    <span class="recorded">Recorded run · nothing here is live</span>
  {/if}
  {#if feedback}<span class="feedback" role="status">{feedback}</span>{/if}
</header>

<style>
  .topbar {
    position: relative;
    display: flex;
    min-height: 56px;
    box-sizing: border-box;
    flex-shrink: 0;
    align-items: center;
    gap: 16px;
    padding: 0 16px;
    color: var(--foreground);
    background: var(--card);
    border-bottom: 1px solid var(--map-line);
  }

  .breadcrumb,
  .states,
  .current,
  .controls,
  .search {
    display: flex;
    align-items: center;
  }

  .breadcrumb {
    gap: 8px;
    min-width: 0;
    font-size: 13px;
    white-space: nowrap;
  }

  .breadcrumb a {
    color: var(--status-live);
    text-decoration: none;
  }

  .breadcrumb strong {
    overflow: hidden;
    max-width: 180px;
    font-size: 15px;
    text-overflow: ellipsis;
  }

  .breadcrumb code,
  .elapsed,
  kbd {
    font-family: var(--font-mono);
  }

  .states {
    gap: 8px;
  }

  .states :global(.status),
  .states :global([data-slot="badge"]) {
    gap: 6px;
  }

  .states :global(.disconnected) {
    color: var(--status-muted);
    border-style: dashed;
  }

  .elapsed,
  .recorded,
  .current {
    color: var(--status-muted);
    font-size: 13px;
  }

  .current {
    gap: 6px;
    padding-left: 8px;
    font: inherit;
    cursor: pointer;
    background: transparent;
    border: 0;
    border-left: 1px solid var(--map-line-soft);
  }

  .current:disabled {
    cursor: default;
  }

  .current:not(:disabled):hover {
    color: var(--foreground);
  }

  .current.waiting {
    color: var(--foreground);
    font-weight: 500;
  }

  .spacer {
    flex: 1;
  }

  .search {
    position: relative;
    width: 240px;
    color: var(--status-muted);
  }

  .search > :global(svg) {
    position: absolute;
    z-index: 1;
    left: 10px;
    pointer-events: none;
  }

  .search :global(input) {
    height: 32px;
    padding-right: 30px;
    padding-left: 30px;
    font-size: 13px;
    border-color: var(--map-line);
  }

  kbd {
    position: absolute;
    right: 8px;
    padding: 0 4px;
    font-size: 11px;
    background: var(--muted);
    border: 1px solid var(--border);
    border-radius: 4px;
  }

  .search-results {
    position: absolute;
    z-index: 30;
    top: calc(100% + 6px);
    right: 0;
    width: 360px;
    max-height: 320px;
    padding: 4px;
    overflow: auto;
    color: var(--foreground);
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 7px;
    box-shadow: var(--shadow-md);
  }

  .search-results button {
    display: flex;
    width: 100%;
    align-items: baseline;
    justify-content: space-between;
    gap: 12px;
    padding: 7px 8px;
    color: inherit;
    font: inherit;
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: 5px;
  }

  .search-results button:hover,
  .search-results button:focus-visible {
    background: var(--muted);
    outline: none;
  }

  .search-results strong {
    overflow: hidden;
    font-family: var(--font-mono);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .search-results span,
  .search-results p {
    color: var(--status-muted);
    font-size: 12px;
  }

  .search-results p {
    margin: 0;
    padding: 8px;
  }

  .controls {
    gap: 8px;
  }

  .controls :global(.cancel) {
    color: var(--destructive);
  }

  .feedback {
    position: absolute;
    top: 100%;
    right: 16px;
    z-index: 20;
    max-width: 360px;
    padding: 5px 9px;
    color: var(--foreground);
    font-size: 13px;
    background: var(--card);
    border: 1px solid var(--border);
    border-radius: 0 0 6px 6px;
    box-shadow: var(--shadow-sm);
  }

  @media (max-width: 1100px) {
    .search,
    .current,
    .elapsed {
      display: none;
    }
  }
</style>
