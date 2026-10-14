<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import { errorMessage } from "#lib/errors";
  import { SvelteMap } from "svelte/reactivity";
  import type { Entry } from "#lib/skgo/promises/types";
  import type { Header } from "#lib/skgo/transcript/types";
  import { setPromiseWithdrawn, watchPromises } from "../workspace.remote";

  let { project, recordings }: { project: string; recordings: Header[] } = $props();
  const root = $derived(`/projects/${project}`);
  const stream = $derived(watchPromises(project));
  const view = $derived(stream.current);
  const entries = $derived(view?.entries ?? []);
  const live = $derived(entries.filter(entry => !entry.withdrawn));
  const tags = $derived([...new Set(entries.flatMap(entry => entry.tags))].sort());
  const orders = [["number", "Number"], ["updated", "Updated"], ["tag", "Tag"]] as const;
  let order = $state<(typeof orders)[number][0]>("number");
  let tag = $state<string>();
  let pending = $state<number>();
  const failures = new SvelteMap<number, string>();
  const smallest = (entry: Entry) => entry.tags.toSorted()[0] ?? "￿";
  const shown = $derived(entries.filter(entry => !tag || entry.tags.includes(tag)).toSorted((a, b) =>
    order === "updated" ? b.updated - a.updated : order === "tag" ? smallest(a).localeCompare(smallest(b)) || a.id - b.id : a.id - b.id));
  const active = $derived(shown.filter(entry => !entry.withdrawn));
  const withdrawn = $derived(shown.filter(entry => entry.withdrawn));
  const day = (ms: number) => new Date(ms).toLocaleDateString(undefined, { month: "short", day: "numeric" });
  const time = (ms: number) => new Date(ms).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  const when = (ms: number) => new Date(ms).toDateString() === new Date().toDateString() ? time(ms) : day(ms);
  function label(id: string) {
    const item = recordings.find(item => item.id === id);
    return item && `${day(item.created)} · ${time(item.created)}`;
  }
  async function withdraw(entry: Entry, value: boolean) {
    pending = entry.id;
    failures.delete(entry.id);
    try { await setPromiseWithdrawn({ project, id: entry.id, withdrawn: value }); }
    catch (error) { if (!navigateOnUnauthorized(error)) failures.set(entry.id, errorMessage(error)); }
    finally { pending = undefined; }
  }
  $effect(() => { if (stream.error) void stream.catch(navigateOnUnauthorized); });
</script>

{#snippet promise(entry: Entry)}
  <details class={["promise", { withdrawn: entry.withdrawn }]}>
    <summary>
      <span class="number">#{entry.id}</span><span class="title">{entry.title}</span>
      {#each entry.tags as name (name)}<span class="tag">{name}</span>{/each}
      <time class="muted" datetime={new Date(entry.updated).toISOString()}>{when(entry.updated)}</time>
    </summary>
    <div class="body">
      <p class="details">{entry.details}</p>
      <p class="muted">Added {day(entry.created)}, {time(entry.created)}{#if entry.updated !== entry.created} · changed {day(entry.updated)}, {time(entry.updated)}{/if}</p>
      {#if entry.evidence.length}
        <h3>From the transcript</h3>
        {#each entry.evidence as quote, i (`${quote.recording}-${quote.chunk}-${i}`)}
          <blockquote><p>“{quote.quote}”</p><a href={`${root}/recordings/${quote.recording}`}>{label(quote.recording) ? `${label(quote.recording)} recording` : "Earlier recording"}</a></blockquote>
        {/each}
      {/if}
      <p class="actions">
        {#if failures.has(entry.id)}<span class="error" role="alert">Could not {entry.withdrawn ? "restore" : "withdraw"} #{entry.id}: {failures.get(entry.id)}</span>{/if}
        <button disabled={pending === entry.id} onclick={() => withdraw(entry, !entry.withdrawn)}>{pending === entry.id ? "Saving…" : entry.withdrawn ? "Restore" : "Withdraw"}</button>
      </p>
    </div>
  </details>
{/snippet}

<section class="ledger" aria-label="Promises">
  <header>
    <h1>Promises</h1>
    {#if entries.length}
      <span class="muted">{tag ? `${active.length} of ${live.length} active · tagged ${tag}` : `${live.length} active · ${entries.length - live.length} withdrawn`}</span>
      <div class="segmented" role="group" aria-label="Sort">
        {#each orders as [value, name] (value)}<button aria-pressed={order === value} onclick={() => order = value}>{name}</button>{/each}
      </div>
    {/if}
  </header>
  {#if !stream.connected && view}<p class="banner">Server disconnected · showing the last loaded promises <button onclick={() => stream.reconnect()}>Reconnect</button></p>{/if}
  {#if view?.working}<p class="working" role="status"><span class="dot" aria-hidden="true"></span>Reading new speech for promises…</p>{/if}
  {#each view?.errors ?? [] as failure (failure.recording)}
    <div class="failure" role="alert"><p>Could not update promises from <a href={`${root}/recordings/${failure.recording}`}>{label(failure.recording) ?? "an earlier recording"}</a>.</p><code>{failure.message}</code></div>
  {/each}
  {#if !view}
    <p class="muted">{stream.error ? "Could not load promises." : "Loading promises…"}</p>
  {:else if !entries.length}
    <div class="empty">
      <h2>No promises yet</h2>
      <p>Promises from requirements discussions appear here as their speech is processed.</p>
      <p>Start one with <strong>Recording ▸ Start Requirements Discussion</strong> in the macOS menu bar, or open a recording and choose <strong>Treat as requirements discussion</strong>.</p>
    </div>
  {:else}
    <div class="chips" role="group" aria-label="Filter by tag">
      <button class="tag" aria-pressed={!tag} onclick={() => tag = undefined}>All</button>
      {#each tags as name (name)}<button class="tag" aria-pressed={tag === name} onclick={() => tag = name}>{name}</button>{/each}
    </div>
    <div class="list">{#each active as entry (entry.id)}{@render promise(entry)}{:else}<p class="muted">No active promises match this filter.</p>{/each}</div>
    <p class="muted hint">To withdraw by voice, say “withdraw promise 2” during a requirements discussion, or use Withdraw.</p>
    {#if withdrawn.length}<details class="withdrawn-group"><summary>Withdrawn ({withdrawn.length})</summary><div class="list">{#each withdrawn as entry (entry.id)}{@render promise(entry)}{/each}</div></details>{/if}
  {/if}
</section>

<style>
  .ledger { height: 100%; overflow: auto; box-sizing: border-box; padding: 32px max(24px, calc((100% - 760px) / 2)); }
  p, h1, h2, h3, blockquote { margin: 0; }
  header { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px 12px; margin-bottom: 14px; }
  h1 { font-size: 1.4rem; font-weight: 600; }
  header .segmented { margin-left: auto; align-self: center; }
  button { font: inherit; cursor: pointer; }
  a { color: inherit; }
  .muted { color: var(--muted-foreground); font-size: 12px; }
  time.muted { font-size: 11px; }
  .working { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; color: var(--muted-foreground); font-size: 12px; }
  .dot { width: 6px; height: 6px; border-radius: 50%; flex: none; background: var(--foreground); }
  .banner { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; color: var(--muted-foreground); font-size: 12px; }
  .failure { display: grid; gap: 2px; margin-bottom: 12px; color: var(--destructive); font-size: 13px; }
  .failure code { color: var(--muted-foreground); font: 11px var(--font-mono); white-space: pre-wrap; }
  .empty { display: grid; gap: 8px; margin-top: 32px; padding: 28px; border: 1px dashed var(--border); border-radius: 10px; color: var(--muted-foreground); }
  .empty h2 { margin-bottom: 4px; color: var(--foreground); font-size: 16px; font-weight: 500; }
  .empty p { max-width: 600px; font-size: 13px; line-height: 1.5; }
  .empty strong { color: var(--foreground); font-weight: 500; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 14px; }
  .tag { padding: 1px 8px; border: 1px solid var(--border); border-radius: 999px; background: var(--muted); color: var(--muted-foreground); font-size: 11px; line-height: 18px; white-space: nowrap; }
  button.tag[aria-pressed="true"] { border-color: var(--primary); background: var(--primary); color: var(--primary-foreground); }
  .segmented { display: flex; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
  .segmented button { padding: 3px 10px; border: 0; background: var(--background); color: var(--muted-foreground); font-size: 12px; }
  .segmented button + button { border-left: 1px solid var(--border); }
  .segmented button[aria-pressed="true"] { background: var(--muted); color: var(--foreground); }
  .list { display: grid; gap: 6px; }
  .hint { margin-top: 12px; }
  .promise { border: 1px solid var(--border); border-radius: 10px; }
  .promise summary { display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px 8px; padding: 10px 14px; border-radius: 10px; cursor: pointer; list-style: none; }
  .promise summary::-webkit-details-marker { display: none; }
  .promise summary::before { content: "›"; display: inline-block; width: 8px; color: var(--muted-foreground); transition: transform .15s; }
  .promise[open] summary::before { transform: rotate(90deg); }
  .promise summary:hover { background: var(--muted); }
  .number { color: var(--muted-foreground); font: 12px var(--font-mono); font-variant-numeric: tabular-nums; }
  .title { flex: 1 1 50%; font-size: 14px; line-height: 1.5; }
  .withdrawn .title, .withdrawn .details { color: var(--muted-foreground); }
  .body { display: grid; gap: 10px; padding: 0 14px 12px 30px; font-size: 13px; }
  .details { line-height: 1.6; white-space: pre-wrap; }
  h3 { color: var(--muted-foreground); font-size: 11px; font-weight: 500; }
  blockquote { display: grid; gap: 2px; padding-left: 10px; border-left: 2px solid var(--border); }
  blockquote p { font-size: 12px; }
  blockquote a { color: var(--muted-foreground); font-size: 11px; }
  blockquote a:hover { color: var(--foreground); }
  .actions { display: flex; align-items: center; justify-content: flex-end; gap: 12px; }
  .actions button, .banner button { padding: 4px 10px; border: 1px solid var(--border); border-radius: 6px; background: var(--background); color: var(--foreground); font-size: 12px; }
  .actions button:disabled { opacity: .5; cursor: default; }
  .error { color: var(--destructive); font-size: 12px; }
  .withdrawn-group { margin-top: 20px; }
  .withdrawn-group > summary { margin-bottom: 8px; color: var(--muted-foreground); font-size: 12px; font-weight: 500; cursor: pointer; }
  @media (prefers-reduced-motion: reduce) { .promise summary::before { transition: none; } }
</style>
