<script lang="ts">
  import { goto, invalidateAll } from "$app/navigation";
  import BotIcon from "@lucide/svelte/icons/bot";
  import GitBranchIcon from "@lucide/svelte/icons/git-branch";
  import LoaderCircleIcon from "@lucide/svelte/icons/loader-circle";
  import MessageSquarePlusIcon from "@lucide/svelte/icons/message-square-plus";
  import SendIcon from "@lucide/svelte/icons/send";
  import TerminalSquareIcon from "@lucide/svelte/icons/square-terminal";
  import { createConversation, sendConversationMessage } from "../conversation.remote.js";
  import type { Conversation, Message } from "#lib/skgo/conversation/types.js";

  let { data }: { data: { items: Conversation[]; selected: Conversation } } = $props();

  const defaults: Record<string, string> = {
    codex: "gpt-5.6-luna",
    claude: "claude-haiku-4-5-20251001",
    agy: "gemini-3.8-flash-low",
  };

  let current = $state<Conversation>();
  let title = $state("");
  let provider = $state("");
  let model = $state("");
  let message = $state("");
  let creating = $state(false);
  let sending = $state(false);
  let feedback = $state("");

  $effect(() => {
    current = data.selected;
  });

  $effect(() => {
    if (data.selected.status !== "working") return;
    const refresh = window.setInterval(() => void invalidateAll(), 750);
    return () => window.clearInterval(refresh);
  });

  function chooseProvider(event: Event) {
    provider = (event.currentTarget as HTMLSelectElement).value;
    model = defaults[provider] ?? "";
  }

  async function create(event: SubmitEvent) {
    event.preventDefault();
    if (!provider || creating) return;
    creating = true;
    feedback = "";
    try {
      const created = await createConversation({ title, provider, model });
      title = "";
      await goto(`/conversations?conversation=${encodeURIComponent(created.id)}`);
      await invalidateAll();
    } catch (error) {
      feedback = error instanceof Error ? error.message : String(error);
    } finally {
      creating = false;
    }
  }

  async function send(event: SubmitEvent) {
    event.preventDefault();
    const text = message.trim();
    if (!current?.id || !text || sending) return;
    message = "";
    feedback = "";
    sending = true;
    const optimistic: Message = { role: "user", text, created: Date.now() };
    current = { ...current, status: "working", error: "", messages: [...current.messages, optimistic] };
    try {
      current = await sendConversationMessage({ conversation: current.id, message: text });
    } catch (error) {
      feedback = error instanceof Error ? error.message : String(error);
    } finally {
      sending = false;
      await invalidateAll();
    }
  }

  const providerName = (name: string) =>
    name === "agy" ? "agy / Gemini" : name === "codex" ? "Codex" : "Claude";
</script>

<svelte:head>
  <title>Conversations — Gimble</title>
  <meta
    name="description"
    content="Provider-selected agent conversations in real Git worktrees."
  />
</svelte:head>

<div class="conversation-page">
  <aside class="sidebar">
    <div class="sidebar-heading">
      <div>
        <p class="eyebrow">Workspace</p>
        <h1>Conversations</h1>
      </div>
      <MessageSquarePlusIcon size={19} aria-hidden="true" />
    </div>

    <form class="new-conversation" onsubmit={create}>
      <label>
        <span>Title <small>optional</small></span>
        <input bind:value={title} placeholder="What are we working on?" />
      </label>
      <label>
        <span>Provider</span>
        <select value={provider} onchange={chooseProvider} required aria-label="Provider">
          <option value="" disabled>Choose a provider</option>
          <option value="codex">Codex</option>
          <option value="claude">Claude</option>
          <option value="agy">agy / Gemini</option>
        </select>
      </label>
      <label>
        <span>Model</span>
        <input bind:value={model} disabled={!provider} placeholder="Choose a provider first" />
      </label>
      <button class="primary" type="submit" disabled={!provider || creating}>
        {#if creating}<LoaderCircleIcon class="spin" size={16} />{/if}
        {creating ? "Creating worktree…" : "New conversation"}
      </button>
    </form>

    <div class="conversation-list" aria-label="Saved conversations">
      {#each data.items as item (item.id)}
        <a
          class:active={item.id === current?.id}
          href={`/conversations?conversation=${encodeURIComponent(item.id)}`}
          data-sveltekit-reload
        >
          <span class="list-title">{item.title}</span>
          <span class="list-meta">
            <span class:working={item.status === "working"}></span>
            {providerName(item.provider)} · {item.messages.length} messages
          </span>
        </a>
      {:else}
        <p class="empty-list">Choose a provider to open the first conversation.</p>
      {/each}
    </div>
  </aside>

  <section class="workspace">
    {#if current?.id}
      <header class="conversation-header">
        <div>
          <div class="title-row">
            <BotIcon size={20} aria-hidden="true" />
            <h2>{current.title}</h2>
            <span class="provider-badge">{providerName(current.provider)}</span>
          </div>
          <p class="model">{current.model}</p>
        </div>
        <div class="git-facts">
          <div><GitBranchIcon size={15} /><code>{current.branch}</code></div>
          <div><TerminalSquareIcon size={15} /><code>{current.worktree}</code></div>
        </div>
      </header>

      <div class="transcript" aria-live="polite">
        {#each current.messages as entry, index (`${entry.created}-${index}`)}
          <article class:assistant={entry.role === "assistant"} class="message">
            <span class="role">{entry.role === "assistant" ? providerName(current.provider) : "You"}</span>
            <p>{entry.text}</p>
          </article>
        {:else}
          <div class="empty-transcript">
            <BotIcon size={30} aria-hidden="true" />
            <h3>Ready in its own worktree</h3>
            <p>Send a message to start a continuing {providerName(current.provider)} session.</p>
          </div>
        {/each}
        {#if sending || current.status === "working"}
          <div class="working-state">
            <LoaderCircleIcon class="spin" size={16} />
            {providerName(current.provider)} is working in {current.branch}
          </div>
        {/if}
        {#if current.error}
          <div class="error-state"><strong>Provider error</strong><span>{current.error}</span></div>
        {/if}
        {#if !current.live}
          <div class="history-state">
            <strong>Saved history</strong>
            <span>The provider session ended when the server restarted. Start a new conversation to continue.</span>
          </div>
        {/if}
      </div>

      <form class="composer" onsubmit={send}>
        <textarea
          bind:value={message}
          rows={3}
          placeholder={`Message ${providerName(current.provider)}…`}
          aria-label="Message"
          disabled={!current.live || sending || current.status === "working"}
        ></textarea>
        <button
          class="send"
          type="submit"
          aria-label="Send message"
          disabled={!current.live || !message.trim() || sending || current.status === "working"}
        >
          <SendIcon size={18} />
        </button>
      </form>
    {:else}
      <div class="no-selection">
        <MessageSquarePlusIcon size={34} aria-hidden="true" />
        <h2>Open a provider conversation</h2>
        <p>Each conversation gets a distinct Git branch and worktree before the first message.</p>
      </div>
    {/if}

    {#if feedback}<div class="feedback" role="alert">{feedback}</div>{/if}
  </section>
</div>

<style>
  .conversation-page {
    display: grid;
    grid-template-columns: 320px minmax(0, 1fr);
    height: calc(100vh - 55px);
    min-height: 620px;
    background: var(--surface);
  }

  .sidebar {
    display: flex;
    min-height: 0;
    flex-direction: column;
    border-right: 1px solid var(--border);
    background: var(--card);
  }

  .sidebar-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 22px 22px 14px;
  }

  .eyebrow {
    margin: 0 0 2px;
    color: var(--muted-foreground);
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
  }

  h1,
  h2,
  h3,
  p { margin: 0; }

  h1 { font-size: 1.25rem; }

  .new-conversation {
    display: grid;
    gap: 11px;
    margin: 0 14px 16px;
    padding: 14px;
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    background: var(--surface);
  }

  label { display: grid; gap: 5px; }

  label > span {
    color: var(--muted-foreground);
    font-size: 0.75rem;
    font-weight: 650;
  }

  small { font-weight: 400; }

  input,
  select,
  textarea {
    box-sizing: border-box;
    width: 100%;
    border: 1px solid var(--input);
    border-radius: calc(var(--radius) - 2px);
    outline: none;
    color: var(--foreground);
    background: var(--card);
    font: inherit;
  }

  input,
  select { height: 36px; padding: 0 10px; font-size: 0.82rem; }

  input:focus,
  select:focus,
  textarea:focus { border-color: var(--ring); box-shadow: 0 0 0 2px color-mix(in oklch, var(--ring) 25%, transparent); }

  button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    border: 0;
    cursor: pointer;
    font: inherit;
    font-weight: 650;
  }

  button:disabled { cursor: default; opacity: 0.5; }

  .primary {
    height: 37px;
    border-radius: calc(var(--radius) - 1px);
    color: var(--primary-foreground);
    background: var(--primary);
    font-size: 0.82rem;
  }

  .conversation-list {
    display: flex;
    min-height: 0;
    flex: 1;
    flex-direction: column;
    gap: 3px;
    overflow: auto;
    padding: 0 10px 18px;
  }

  .conversation-list a {
    display: grid;
    gap: 4px;
    padding: 11px 12px;
    border-radius: calc(var(--radius) - 1px);
    color: inherit;
    text-decoration: none;
  }

  .conversation-list a:hover,
  .conversation-list a.active { background: var(--accent); }

  .list-title { overflow: hidden; font-size: 0.85rem; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }

  .list-meta {
    display: flex;
    align-items: center;
    gap: 5px;
    color: var(--muted-foreground);
    font-size: 0.7rem;
  }

  .list-meta > span { width: 6px; height: 6px; border-radius: 999px; background: var(--map-line); }
  .list-meta > span.working { background: var(--status-running); }
  .empty-list { padding: 12px; color: var(--muted-foreground); font-size: 0.8rem; }

  .workspace {
    position: relative;
    display: grid;
    min-width: 0;
    min-height: 0;
    grid-template-rows: auto minmax(0, 1fr) auto;
    background: var(--background);
  }

  .conversation-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 24px;
    padding: 20px 28px;
    border-bottom: 1px solid var(--border);
  }

  .title-row { display: flex; align-items: center; gap: 9px; }
  .title-row h2 { font-size: 1.03rem; }
  .model { margin: 4px 0 0 29px; color: var(--muted-foreground); font-family: var(--font-mono); font-size: 0.72rem; }

  .provider-badge {
    padding: 2px 8px;
    border: 1px solid var(--border);
    border-radius: 999px;
    color: var(--muted-foreground);
    background: var(--surface);
    font-size: 0.67rem;
    font-weight: 700;
  }

  .git-facts { display: grid; max-width: 52%; gap: 5px; }
  .git-facts div { display: flex; align-items: center; justify-content: flex-end; gap: 7px; color: var(--muted-foreground); }
  code { overflow: hidden; font-family: var(--font-mono); font-size: 0.68rem; text-overflow: ellipsis; white-space: nowrap; }

  .transcript {
    display: flex;
    flex-direction: column;
    gap: 18px;
    overflow: auto;
    padding: 34px max(28px, calc((100% - 780px) / 2));
  }

  .message { align-self: flex-end; max-width: min(680px, 86%); }
  .message.assistant { align-self: flex-start; width: min(680px, 92%); }
  .role { display: block; margin-bottom: 5px; color: var(--muted-foreground); font-size: 0.7rem; font-weight: 700; }
  .message p { padding: 11px 14px; border-radius: 16px 16px 4px 16px; background: var(--accent); white-space: pre-wrap; }
  .message.assistant p { padding: 0; border-radius: 0; background: transparent; }

  .working-state,
  .error-state,
  .history-state {
    display: flex;
    width: fit-content;
    align-items: center;
    gap: 8px;
    color: var(--muted-foreground);
    font-size: 0.78rem;
  }

  .error-state { align-items: flex-start; flex-direction: column; gap: 2px; padding: 10px 12px; border: 1px solid color-mix(in oklch, var(--destructive) 35%, var(--border)); border-radius: var(--radius); color: var(--destructive); }
  .history-state { align-items: flex-start; flex-direction: column; gap: 2px; padding: 10px 12px; border: 1px solid var(--border); border-radius: var(--radius); background: var(--surface); }

  .empty-transcript,
  .no-selection {
    display: grid;
    place-items: center;
    align-content: center;
    gap: 8px;
    height: 100%;
    color: var(--muted-foreground);
    text-align: center;
  }

  .empty-transcript h3,
  .no-selection h2 { color: var(--foreground); }
  .empty-transcript p,
  .no-selection p { max-width: 480px; font-size: 0.86rem; }

  .composer {
    position: relative;
    width: min(780px, calc(100% - 56px));
    margin: 0 auto 25px;
  }

  textarea { min-height: 82px; resize: vertical; padding: 13px 50px 13px 14px; box-shadow: var(--shadow-md); }
  .send { position: absolute; right: 10px; bottom: 10px; width: 34px; height: 34px; border-radius: 9px; color: var(--primary-foreground); background: var(--primary); }
  .feedback { position: absolute; right: 24px; bottom: 20px; max-width: 480px; padding: 10px 12px; border-radius: var(--radius); color: var(--destructive-foreground); background: var(--destructive); font-size: 0.78rem; box-shadow: var(--shadow-lg); }
  :global(.spin) { animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  @media (max-width: 800px) {
    .conversation-page { grid-template-columns: 250px minmax(0, 1fr); }
    .conversation-header { flex-direction: column; }
    .git-facts { max-width: 100%; }
    .git-facts div { justify-content: flex-start; }
  }
</style>
