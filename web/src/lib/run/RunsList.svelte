<script module lang="ts">
  import type { InterviewRow, RunRow } from "../observation/index.js";

  export type RunsFilter = "all" | "active" | "needs-answer" | "ended" | "failed" | "cancelled";

  export type RunsListProps = {
    runs: RunRow[];
    attention: InterviewRow[];
    summaries: Record<string, string>;
    elapsed: Record<string, string>;
    now?: number;
    onopenrun?: (run: RunRow) => void;
  };
</script>

<script lang="ts">
  import ChevronRightIcon from "@lucide/svelte/icons/chevron-right";
  import MessageCircleQuestionIcon from "@lucide/svelte/icons/message-circle-question-mark";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  import * as Table from "#lib/components/ui/table/index.js";
  import Pip from "./Pip.svelte";

  let {
    runs,
    attention,
    summaries,
    elapsed,
    now = Date.now(),
    onopenrun,
  }: RunsListProps = $props();

  let filter = $state<RunsFilter>("all");
  let query = $state("");

  const pending = $derived(attention.filter((question) => question.status === "pending"));
  const pendingRunIDs = $derived(new Set(pending.map((question) => question.run)));
  const counts = $derived.by(() => ({
    all: runs.length,
    active: runs.filter((run) => run.status === "running").length,
    "needs-answer": runs.filter((run) => pendingRunIDs.has(run.id)).length,
    ended: runs.filter((run) => run.status === "completed").length,
    failed: runs.filter((run) => run.status === "failed").length,
    cancelled: runs.filter((run) => run.status === "cancelled").length,
  }));
  const filteredRuns = $derived.by(() => {
    const needle = query.trim().toLocaleLowerCase();
    return runs.filter((run) => {
      const matchesFilter =
        filter === "all" ||
        (filter === "active" && run.status === "running") ||
        (filter === "needs-answer" && pendingRunIDs.has(run.id)) ||
        (filter === "ended" && run.status === "completed") ||
        run.status === filter;
      const matchesQuery =
        !needle ||
        run.name.toLocaleLowerCase().includes(needle) ||
        run.id.toLocaleLowerCase().includes(needle);
      return matchesFilter && matchesQuery;
    });
  });

  const filters: { value: RunsFilter; label: string }[] = [
    { value: "all", label: "All" },
    { value: "active", label: "Active" },
    { value: "needs-answer", label: "Needs answer" },
    { value: "ended", label: "Ended" },
    { value: "failed", label: "Failed" },
    { value: "cancelled", label: "Cancelled" },
  ];

  function openRun(runID: string) {
    const run = runs.find((candidate) => candidate.id === runID);
    if (run) onopenrun?.(run);
  }

  function relativeTime(timestamp: number) {
    const seconds = Math.max(0, Math.floor((now - timestamp) / 1000));
    if (seconds < 60) return `${seconds} s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes} min ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours} h ago`;
    return new Intl.DateTimeFormat("en", { month: "short", day: "numeric" }).format(timestamp);
  }

  function statusLabel(run: RunRow) {
    return run.status === "completed" ? "Ended" : run.status[0].toUpperCase() + run.status.slice(1);
  }

  function pipState(run: RunRow) {
    if (run.status === "running") return "running" as const;
    if (run.status === "failed") return "failed" as const;
    return "ended" as const;
  }

  function pendingCount(runID: string) {
    return pending.filter((question) => question.run === runID).length;
  }

  function summaryDetail(run: RunRow) {
    const count = pendingCount(run.id);
    const summary = summaries[run.id] ?? (run.error || "Recorded run");
    if (count === 0) return summary;
    return summary.replace(/^\d+ questions? waiting\s*·\s*/, "");
  }
</script>

<section class="runs-list" aria-labelledby="runs-title">
  <header class="heading">
    <h1 id="runs-title">Runs</h1>
    <p>Everything this project runtime has recorded. Current work first.</p>
  </header>

  {#if pending.length > 0}
    <section class="attention" aria-labelledby="attention-title">
      <h2 id="attention-title">Needs your answer · {pending.length}</h2>
      <div class="attention-grid">
        {#each pending as question (question.question_id)}
          <button class="attention-item" type="button" onclick={() => openRun(question.run)}>
            <span class="attention-icon"><MessageCircleQuestionIcon size={16} /></span>
            <span class="attention-copy">
              <span class="attention-title">
                <strong>{runs.find((run) => run.id === question.run)?.name ?? question.run}</strong>
                <span>{question.scope} · {question.name}</span>
              </span>
              <span class="question">{question.question}</span>
            </span>
            <span class="attention-actions">
              <span>{relativeTime(question.asked)}</span>
              <span class="answer-label">Answer</span>
            </span>
          </button>
        {/each}
      </div>
    </section>
  {/if}

  <section class="recorded" aria-label="Recorded runs">
    <div class="tools">
      <div class="filters" role="group" aria-label="Filter runs">
        {#each filters as item}
          <Button
            size="sm"
            variant={filter === item.value ? "secondary" : "outline"}
            aria-pressed={filter === item.value}
            onclick={() => (filter = item.value)}
          >
            {item.label} · {counts[item.value]}
          </Button>
        {/each}
      </div>
      <label class="search">
        <span>Search runs</span>
        <Input type="search" placeholder="Search by workflow or run id" bind:value={query} />
      </label>
    </div>

    <div class="table-shell">
      <Table.Root>
        <Table.Header>
          <Table.Row>
            <Table.Head class="workflow-column">Workflow</Table.Head>
            <Table.Head class="status-column">Status</Table.Head>
            <Table.Head>Now, or how it ended</Table.Head>
            <Table.Head class="started-column">Started</Table.Head>
            <Table.Head class="duration-column">Duration</Table.Head>
            <Table.Head class="open-column"><span class="sr-only">Open run</span></Table.Head>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {#each filteredRuns as run (run.id)}
            <Table.Row>
              <Table.Cell>
                <button class="run-link" type="button" onclick={() => onopenrun?.(run)}>
                  <strong>{run.name}</strong>
                  <span>{run.id}</span>
                </button>
              </Table.Cell>
              <Table.Cell>
                <Badge variant={run.status === "failed" ? "destructive" : "outline"} class="status">
                  {#if run.status !== "failed"}<Pip state={pipState(run)} />{/if}
                  {statusLabel(run)}
                </Badge>
              </Table.Cell>
              <Table.Cell>
                <div class="summary">
                  {#if pendingCount(run.id) > 0}
                    <Badge>{pendingCount(run.id)} questions waiting</Badge>
                  {/if}
                  <span>{summaryDetail(run)}</span>
                </div>
              </Table.Cell>
              <Table.Cell class="secondary">{relativeTime(run.started)}</Table.Cell>
              <Table.Cell class="duration">{elapsed[run.id] ?? "—"}</Table.Cell>
              <Table.Cell>
                <button class="open-run" type="button" aria-label={`Open ${run.name} ${run.id}`} onclick={() => onopenrun?.(run)}>
                  <ChevronRightIcon size={14} />
                </button>
              </Table.Cell>
            </Table.Row>
          {:else}
            <Table.Row>
              <Table.Cell colspan={6} class="no-results">No runs match this filter.</Table.Cell>
            </Table.Row>
          {/each}
        </Table.Body>
      </Table.Root>
    </div>
    <p class="footnote">
      Runs are recorded tables on disk. Opening an older run reads its record; nothing is replayed.
      “Needs answer” comes from pending questions, not from a run status.
    </p>
  </section>
</section>

<style>
  .runs-list {
    box-sizing: border-box;
    width: min(1120px, 100%);
    padding: 36px 0 24px;
    color: var(--foreground);
  }

  .heading,
  .attention,
  .recorded {
    display: flex;
    flex-direction: column;
  }

  .heading {
    gap: 4px;
    margin-bottom: 28px;
  }

  h1,
  h2,
  p {
    margin: 0;
  }

  h1 {
    font-size: 24px;
    font-weight: 600;
    line-height: 32px;
    letter-spacing: -0.025em;
  }

  .heading p,
  .summary span,
  .footnote,
  .attention-title span,
  .attention-actions {
    color: var(--status-muted);
  }

  .attention {
    gap: 10px;
    margin-bottom: 28px;
  }

  h2 {
    font-size: 13px;
    font-weight: 600;
    line-height: 18px;
    letter-spacing: 0.01em;
  }

  .attention-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .attention-item {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    color: var(--foreground);
    text-align: left;
    cursor: pointer;
    background: var(--card);
    border: 1px solid var(--map-line);
    border-radius: calc(var(--radius) - 2px);
  }

  .attention-item:hover,
  .attention-item:focus-visible {
    background: var(--muted);
    outline: none;
    box-shadow: 0 0 0 3px color-mix(in oklch, var(--status-running) 22%, transparent);
  }

  .attention-icon {
    display: inline-flex;
    width: 32px;
    height: 32px;
    flex: 0 0 auto;
    align-items: center;
    justify-content: center;
    background: var(--muted);
    border-radius: calc(var(--radius) - 4px);
  }

  .attention-copy {
    display: flex;
    min-width: 0;
    flex: 1;
    flex-direction: column;
    gap: 2px;
  }

  .attention-title {
    display: flex;
    min-width: 0;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    line-height: 18px;
  }

  .attention-title span,
  .question {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .attention-title span,
  .attention-actions,
  .run-link span {
    font-family: var(--font-mono);
    font-size: 13px;
  }

  .question {
    font-size: 14px;
    line-height: 20px;
  }

  .attention-actions {
    display: flex;
    flex: 0 0 auto;
    align-items: center;
    gap: 8px;
  }

  .answer-label {
    padding: 3px 8px;
    color: var(--primary-foreground);
    font-family: var(--font-sans);
    font-size: 12px;
    font-weight: 500;
    background: var(--primary);
    border-radius: calc(var(--radius) - 4px);
  }

  .recorded {
    gap: 12px;
  }

  .tools,
  .filters,
  .summary {
    display: flex;
    align-items: center;
  }

  .tools {
    gap: 12px;
  }

  .filters {
    gap: 0;
  }

  :global(.filters button) {
    border-radius: 0;
  }

  :global(.filters button:first-child) {
    border-radius: calc(var(--radius) - 2px) 0 0 calc(var(--radius) - 2px);
  }

  :global(.filters button:last-child) {
    border-radius: 0 calc(var(--radius) - 2px) calc(var(--radius) - 2px) 0;
  }

  :global(.filters button + button) {
    margin-left: -1px;
  }

  .search {
    width: 300px;
    margin-left: auto;
  }

  .search span,
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  .table-shell {
    overflow: hidden;
    background: var(--card);
    border: 1px solid var(--map-line);
    border-radius: var(--radius);
  }

  :global(.table-shell [data-slot="table-container"]) {
    overflow-x: auto;
  }

  :global(.table-shell th),
  :global(.table-shell td) {
    padding: 10px 12px;
  }

  :global(.table-shell tbody tr:last-child) {
    border-bottom: 0;
  }

  :global(.table-shell .workflow-column) {
    width: 260px;
  }

  :global(.table-shell .status-column) {
    width: 130px;
  }

  :global(.table-shell .started-column) {
    width: 150px;
  }

  :global(.table-shell .duration-column) {
    width: 90px;
    text-align: right;
  }

  :global(.table-shell .open-column) {
    width: 40px;
  }

  .run-link {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 0;
    color: var(--foreground);
    text-align: left;
    cursor: pointer;
    background: transparent;
    border: 0;
  }

  .run-link strong {
    color: var(--status-live);
    font-weight: 500;
  }

  .run-link:hover strong,
  .run-link:focus-visible strong {
    text-decoration: underline;
  }

  .run-link span {
    color: var(--status-muted);
  }

  :global(.status) {
    gap: 6px;
  }

  .summary {
    min-width: 280px;
    gap: 8px;
    font-size: 13px;
  }

  :global(.table-shell .secondary),
  :global(.table-shell .duration) {
    color: var(--status-muted);
  }

  :global(.table-shell .duration) {
    font-family: var(--font-mono);
    font-size: 13px;
    color: var(--status-muted);
    text-align: right;
  }

  .open-run {
    display: inline-flex;
    padding: 5px;
    color: var(--status-muted);
    cursor: pointer;
    background: transparent;
    border: 0;
    border-radius: calc(var(--radius) - 4px);
  }

  .open-run:hover,
  .open-run:focus-visible {
    color: var(--foreground);
    background: var(--muted);
  }

  :global(.table-shell .no-results) {
    height: 96px;
    color: var(--status-muted);
    text-align: center;
  }

  .footnote {
    font-size: 13px;
    line-height: 18px;
  }

  @media (max-width: 900px) {
    .runs-list {
      padding: 24px 16px;
    }

    .attention-grid {
      grid-template-columns: 1fr;
    }

    .tools {
      align-items: stretch;
      flex-direction: column;
    }

    .filters {
      overflow-x: auto;
    }

    .search {
      width: 100%;
      margin-left: 0;
    }
  }
</style>
