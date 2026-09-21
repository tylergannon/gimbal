<script module lang="ts">
  import type { InterviewRow, RunRow } from "../observation/index.js";

  export type RunsFilter = "all" | "active" | "needs-answer" | "ended" | "failed" | "cancelled";

  export type RunCardItem = {
    run: RunRow;
    summary: string;
    elapsed: string;
    activity_at: number;
    cost: string;
    session_count: number;
    turn_count: number;
    instruction: string;
  };

  export type RunsListProps = {
    items: RunCardItem[];
    attention: InterviewRow[];
    now?: number;
  };
</script>

<script lang="ts">
  import ArrowUpRightIcon from "@lucide/svelte/icons/arrow-up-right";
  import MessageCircleQuestionIcon from "@lucide/svelte/icons/message-circle-question-mark";
  import { Badge } from "#lib/components/ui/badge/index.js";
  import { Button } from "#lib/components/ui/button/index.js";
  import { Input } from "#lib/components/ui/input/index.js";
  import Pip from "./Pip.svelte";

  let {
    items,
    attention,
    now = Date.now(),
  }: RunsListProps = $props();

  let filter = $state<RunsFilter>("all");
  let query = $state("");

  const runs = $derived(items.map((item) => item.run));
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
  const filteredItems = $derived.by(() => {
    const needle = query.trim().toLocaleLowerCase();
    return items.filter((item) => {
      const run = item.run;
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

  function relativeTime(timestamp: number) {
    if (timestamp === 0) return "No activity yet";
    const seconds = Math.max(0, Math.floor((now - timestamp) / 1000));
    if (seconds < 60) return `${seconds} s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes} min ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours} h ago`;
    const date = new Date(timestamp);
    const month = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"][date.getMonth()];
    return `${month} ${date.getDate()}`;
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

  function pendingLabel(runID: string) {
    const count = pendingCount(runID);
    return `${count} ${count === 1 ? "question" : "questions"} waiting`;
  }

  function summaryDetail(item: RunCardItem) {
    const run = item.run;
    const count = pendingCount(run.id);
    const summary = item.summary || run.error || "Recorded run";
    if (count === 0) return summary;
    return summary.replace(/^\d+ questions? waiting\s*·\s*/, "");
  }

  function activityTitle(timestamp: number) {
    return timestamp === 0 ? undefined : new Date(timestamp).toLocaleString();
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
          <a class="attention-item" href={`/runs/${encodeURIComponent(question.run)}`}>
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
          </a>
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

    {#if filteredItems.length > 0}
      <div class="cards">
        {#each filteredItems as item (item.run.id)}
          <a
            class="run-card"
            class:live={item.run.status === "running"}
            aria-label={`Open ${item.run.name} ${item.run.id}`}
            href={`/runs/${encodeURIComponent(item.run.id)}`}
          >
            <span class="card-head">
              <span class="identity">
                <strong>{item.run.name}</strong>
                <code>{item.run.id}</code>
              </span>
              <Badge
                variant={item.run.status === "failed" ? "destructive" : "outline"}
                class="status"
              >
                {#if item.run.status !== "failed"}<Pip state={pipState(item.run)} />{/if}
                {statusLabel(item.run)}
              </Badge>
            </span>

            <span class="latest">
              <span class="activity">
                <span>Latest activity</span>
                <time
                  datetime={item.activity_at ? new Date(item.activity_at).toISOString() : undefined}
                  title={activityTitle(item.activity_at)}
                >{relativeTime(item.activity_at)}</time>
              </span>
              <span class="summary">
                {#if pendingCount(item.run.id) > 0}
                  <Badge>{pendingLabel(item.run.id)}</Badge>
                {/if}
                <span>{summaryDetail(item)}</span>
              </span>
            </span>

            <span class="instruction">
              <span>Latest instruction</span>
              {#if item.instruction}
                <span class="prompt">{item.instruction}</span>
              {:else}
                <span class="prompt no-instruction">No instruction recorded yet.</span>
              {/if}
            </span>

            <span class="card-foot">
              <span class="stats">
                <span><span>Total cost</span><strong>{item.cost}</strong></span>
                <span><span>Elapsed</span><strong>{item.elapsed}</strong></span>
                <span><span>Sessions</span><strong>{item.session_count}</strong></span>
                <span><span>Turns</span><strong>{item.turn_count}</strong></span>
              </span>
              <span class="open-label">Open run <ArrowUpRightIcon size={14} /></span>
            </span>
          </a>
        {/each}
      </div>
    {:else}
      <div class="no-results" role="status">No runs match this filter or search.</div>
    {/if}
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
  .attention-actions {
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
  .filters {
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

  .search > span {
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

  .cards {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
  }

  .run-card {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 16px;
    padding: 16px;
    color: var(--foreground);
    font: inherit;
    text-align: left;
    cursor: pointer;
    background: var(--card);
    border: 1px solid var(--map-line);
    border-radius: var(--radius);
    box-shadow: 0 1px 2px rgb(0 0 0 / 6%);
    transition:
      border-color 120ms ease,
      box-shadow 120ms ease,
      transform 120ms ease;
  }

  .run-card.live {
    border-color: color-mix(in oklch, var(--status-running) 45%, var(--map-line));
  }

  .run-card:hover,
  .run-card:focus-visible {
    border-color: var(--map-line-strong);
    outline: none;
    box-shadow:
      0 0 0 3px color-mix(in oklch, var(--status-running) 14%, transparent),
      0 4px 12px rgb(0 0 0 / 8%);
    transform: translateY(-1px);
  }

  .card-head,
  .card-foot,
  .activity,
  .summary,
  .stats,
  .open-label {
    display: flex;
    align-items: center;
  }

  .card-head,
  .card-foot {
    justify-content: space-between;
    gap: 12px;
  }

  .identity,
  .latest,
  .instruction {
    display: flex;
    min-width: 0;
    flex-direction: column;
  }

  .identity {
    gap: 2px;
  }

  .identity strong {
    overflow: hidden;
    font-size: 16px;
    line-height: 22px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .identity code,
  .stats strong,
  .activity time {
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .identity code,
  .activity,
  .instruction > span:first-child,
  .stats > span > span,
  .footnote {
    color: var(--status-muted);
  }

  :global(.status) {
    gap: 6px;
  }

  .latest {
    gap: 7px;
  }

  .activity {
    justify-content: space-between;
    gap: 12px;
    font-size: 12px;
    line-height: 16px;
  }

  .summary {
    min-height: 22px;
    gap: 8px;
    font-size: 13px;
    line-height: 18px;
  }

  .summary > span:last-child {
    display: -webkit-box;
    min-width: 0;
    overflow: hidden;
    overflow-wrap: anywhere;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }

  .instruction {
    gap: 5px;
    padding: 10px 12px;
    background: color-mix(in oklch, var(--muted) 62%, transparent);
    border-radius: calc(var(--radius) - 3px);
  }

  .instruction > span:first-child,
  .stats > span > span {
    font-size: 11px;
    font-weight: 600;
    line-height: 15px;
    letter-spacing: 0.03em;
    text-transform: uppercase;
  }

  .prompt {
    display: -webkit-box;
    min-height: 40px;
    overflow: hidden;
    font-size: 13px;
    line-height: 20px;
    white-space: pre-wrap;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
  }

  .prompt.no-instruction {
    color: var(--status-muted);
    font-style: italic;
  }

  .stats {
    min-width: 0;
    gap: 16px;
  }

  .stats > span {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .stats strong {
    font-weight: 500;
  }

  .open-label {
    flex: 0 0 auto;
    gap: 4px;
    color: var(--status-live);
    font-size: 12px;
    font-weight: 600;
  }

  .no-results {
    display: grid;
    min-height: 160px;
    place-items: center;
    color: var(--status-muted);
    background: var(--card);
    border: 1px dashed var(--map-line);
    border-radius: var(--radius);
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

  @media (max-width: 760px) {
    .cards {
      grid-template-columns: 1fr;
    }

    .card-foot {
      align-items: flex-end;
    }

    .stats {
      display: grid;
      flex: 1;
      grid-template-columns: repeat(2, minmax(64px, 1fr));
      gap: 8px 16px;
    }
  }
</style>
