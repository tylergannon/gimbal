<script lang="ts">
  import { goto } from "$app/navigation";
  import { page } from "$app/state";
  import LoaderCircleIcon from "@lucide/svelte/icons/loader-circle";
  import MessageSquarePlusIcon from "@lucide/svelte/icons/message-square-plus";
  import { createConversation } from "../conversation.remote.js";
  import type { Conversation } from "#lib/skgo/conversation/types.js";

  let { data, children }: { data: { items: Conversation[] }; children: import("svelte").Snippet } = $props();
  const defaults: Record<string, string> = { codex: "gpt-5.6-luna", claude: "claude-haiku-4-5-20251001", agy: "gemini-3.8-flash-low" };
  let provider = $state("");
  let model = $state("");

  $effect(() => {
    if (createConversation.result) void goto(`/conversations/${encodeURIComponent(createConversation.result.id)}`);
  });
  const providerName = (name: string) => name === "agy" ? "agy / Gemini" : name === "codex" ? "Codex" : "Claude";
</script>

<svelte:head><title>Conversations — Gimble</title><meta name="description" content="Provider-selected agent conversations in real Git worktrees." /></svelte:head>

<div class="conversation-page">
  <aside class="sidebar">
    <div class="sidebar-heading"><div><p class="eyebrow">Workspace</p><h1>Conversations</h1></div><MessageSquarePlusIcon size={19} aria-hidden="true" /></div>
    <form class="new-conversation" {...createConversation}>
      <label><span>Title <small>optional</small></span><input {...createConversation.fields.title.as("text")} placeholder="What are we working on?" />{#each createConversation.fields.title.issues() ?? [] as issue}<p class="issue">{issue.message}</p>{/each}</label>
      <label>
        <span>Provider</span>
        <select {...createConversation.fields.provider.as("select")} bind:value={() => provider, (value) => { provider = value; model = defaults[value] ?? ""; }} required aria-label="Provider">
          <option value="" disabled>Choose a provider</option><option value="codex">Codex</option><option value="claude">Claude</option><option value="agy">agy / Gemini</option>
        </select>
        {#each createConversation.fields.provider.issues() ?? [] as issue}<p class="issue">{issue.message}</p>{/each}
      </label>
      <label><span>Model</span><input {...createConversation.fields.model.as("text")} bind:value={model} disabled={!provider} placeholder="Choose a provider first" />{#each createConversation.fields.model.issues() ?? [] as issue}<p class="issue">{issue.message}</p>{/each}</label>
      {#each createConversation.fields.allIssues() ?? [] as issue}<p class="issue">{issue.message}</p>{/each}
      <button class="primary" type="submit" disabled={!provider || createConversation.pending > 0}>{#if createConversation.pending > 0}<LoaderCircleIcon class="spin" size={16} />{/if}{createConversation.pending > 0 ? "Creating worktree…" : "New conversation"}</button>
    </form>
    <div class="conversation-list" aria-label="Saved conversations">
      {#each data.items as item (item.id)}
        <a class={{ active: page.url.pathname === `/conversations/${item.id}` }} href={`/conversations/${item.id}`}><span class="list-title">{item.title}</span><span class="list-meta"><span class={{ working: item.status === "working" || item.runs.some((run) => run.status === "running") }}></span>{providerName(item.provider)} · {item.messages.length} messages · {item.runs.length} runs</span></a>
      {:else}<p class="empty-list">Choose a provider to open the first conversation.</p>{/each}
    </div>
  </aside>
  <div class="workspace">{@render children()}</div>
</div>

<style>
  .conversation-page { display: grid; grid-template-columns: 320px minmax(0, 1fr); height: calc(100vh - 55px); min-height: 620px; background: var(--surface); }
  .sidebar { display: flex; min-height: 0; flex-direction: column; border-right: 1px solid var(--border); background: var(--card); } .sidebar-heading { display: flex; align-items: center; justify-content: space-between; padding: 22px 22px 14px; } .eyebrow { margin: 0 0 2px; color: var(--muted-foreground); font-size: .7rem; font-weight: 700; letter-spacing: .12em; text-transform: uppercase; } h1, p { margin: 0; } h1 { font-size: 1.25rem; }
  .new-conversation { display: grid; gap: 11px; margin: 0 14px 16px; padding: 14px; border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface); } label { display: grid; gap: 5px; } label > span { color: var(--muted-foreground); font-size: .75rem; font-weight: 650; } small { font-weight: 400; } input, select { box-sizing: border-box; width: 100%; height: 36px; padding: 0 10px; border: 1px solid var(--input); border-radius: calc(var(--radius) - 2px); outline: none; color: var(--foreground); background: var(--card); font: inherit; font-size: .82rem; } input:focus, select:focus { border-color: var(--ring); box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 25%, transparent); } button { display: inline-flex; align-items: center; justify-content: center; gap: 7px; border: 0; cursor: pointer; font: inherit; font-weight: 650; } button:disabled { cursor: default; opacity: .5; } .primary { height: 37px; border-radius: calc(var(--radius) - 1px); color: var(--primary-foreground); background: var(--primary); font-size: .82rem; } .issue { color: var(--destructive); font-size: .72rem; }
  .conversation-list { display: flex; min-height: 0; flex: 1; flex-direction: column; gap: 3px; overflow: auto; padding: 0 10px 18px; } .conversation-list a { display: grid; gap: 4px; padding: 11px 12px; border-radius: calc(var(--radius) - 1px); color: inherit; text-decoration: none; } .conversation-list a:hover, .conversation-list a.active { background: var(--accent); } .list-title { overflow: hidden; font-size: .85rem; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; } .list-meta { display: flex; align-items: center; gap: 5px; color: var(--muted-foreground); font-size: .7rem; } .list-meta > span { width: 6px; height: 6px; border-radius: 999px; background: var(--map-line); } .list-meta > span.working { background: var(--status-running); } .empty-list { padding: 12px; color: var(--muted-foreground); font-size: .8rem; }
  .workspace { position: relative; display: grid; min-width: 0; min-height: 0; background: var(--background); } :global(.spin) { animation: spin 1s linear infinite; } @keyframes spin { to { transform: rotate(360deg); } } @media (max-width: 800px) { .conversation-page { grid-template-columns: 250px minmax(0, 1fr); } }
</style>
