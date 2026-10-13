<script lang="ts">
  import { navigateOnUnauthorized } from "#lib/auth/unauthorized";
  import BotIcon from '@lucide/svelte/icons/bot';
  import ExternalLinkIcon from '@lucide/svelte/icons/external-link';
  import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
  import MessageSquarePlusIcon from '@lucide/svelte/icons/message-square-plus';
  import SendIcon from '@lucide/svelte/icons/send';
  import { sendConversationMessage, projectConversations } from '../../routes/conversation.remote';
  import type { Conversation } from '#lib/skgo/conversation/types';
  let { project, current }: { project: string; current: Conversation } = $props();
  const form = $derived(sendConversationMessage.for(`${project}/${current.id}`));
  const providerName = (name: string) =>
    name === "agy" ? "agy / Gemini" : name === "codex" ? "Codex" : "Claude";
  let actionError = $state("");
</script>
<section class="workspace" aria-label={current.title}>
  {#if actionError}<p role="alert">{actionError}</p>{/if}
    {#if current?.id}
      <div class="transcript" aria-live="polite">
        {#if current.runs.length > 0}
          <section class="conversation-runs" aria-label="Workflow runs">
            <span class="role">Workflow runs</span>
            {#each current.runs as run (run.id)}
			  <a href={`/projects/${project}/runs/${encodeURIComponent(run.id)}`}>
                <span class="run-name">{run.workflow}</span>
                <span class:running={run.status === "running"} class:error={run.status === "error"} class="run-status">
                  {run.status}
                </span>
                <code>{run.id}</code>
                <ExternalLinkIcon size={14} aria-hidden="true" />
              </a>
              {#if run.error}<p class="run-error">{run.error}</p>{/if}
            {/each}
          </section>
        {/if}
        {#each current.messages as entry, index (`${entry.created}-${index}`)}
          <article class:assistant={entry.role === "assistant"} class="message">
            <span class="role">{entry.role === "assistant" ? providerName(current.provider) : "You"}</span>
            <p>{entry.text}</p>
          </article>
        {:else}
          <div class="empty-transcript">
            <BotIcon size={30} aria-hidden="true" />
            <h3>Ready in its working directory</h3>
            <p>Send a message to start a continuing {providerName(current.provider)} session.</p>
          </div>
        {/each}
        {#if !!form.pending || current.status === "working"}
          <div class="working-state">
            <LoaderCircleIcon class="spin" size={16} />
            {providerName(current.provider)} is working in {current.branch || current.worktree}
          </div>
        {/if}
        {#if current.error}
          <div class="error-state"><strong>Conversation error</strong><span>{current.error}</span></div>
        {/if}
        {#if !current.live}
          <div class="history-state">
            <strong>Saved history</strong>
            <span>The provider session ended when the server restarted. Start a new conversation to continue.</span>
          </div>
        {/if}
      </div>

      <form class="composer" {...form.enhance(async (submitted) => {
        try {
          if (await submitted.submit().updates(projectConversations(project))) submitted.element.reset();
        } catch (error) {
          if (!navigateOnUnauthorized(error)) actionError = error instanceof Error ? error.message : String(error);
        }
      })}>
        <input {...form.fields.conversation.as('hidden', current.id)} />
        <textarea {...form.fields.message.as('text')} rows={3} placeholder={`Message ${providerName(current.provider)}…`} aria-label={`Message ${current.title}`} disabled={!current.live || !!form.pending || current.status === 'working'}></textarea>
        <div class="composer-meta"><span>{providerName(current.provider)} · {current.model}</span><code title={current.worktree}>{current.branch || current.worktree}</code></div>
        <button class="send" aria-label="Send message" disabled={!current.live || !!form.pending || current.status === 'working'}><SendIcon size={18} /></button>
        {#each form.fields.allIssues() ?? [] as issue}<p role="alert">{issue.message}</p>{/each}
      </form>
    {:else}
      <div class="no-selection">
        <MessageSquarePlusIcon size={34} aria-hidden="true" />
        <h2>Open a provider conversation</h2>
        <p>Git projects get a distinct branch and worktree. Other projects share their directory between conversations.</p>
      </div>
    {/if}

</section>
<style>
  .workspace { height: 100%; display: flex; flex-direction: column; min-width: 0; min-height: 0; }
  p, h2, h3 { margin: 0; }
  .transcript { flex: 1; min-height: 0; display: flex; flex-direction: column; gap: 22px; overflow: auto; padding: 32px max(24px, calc((100% - 720px) / 2)); }
  .message { align-self: flex-end; max-width: min(560px, 86%); }
  .message.assistant { align-self: flex-start; width: min(680px, 92%); }
  .role { display: block; margin-bottom: 6px; color: var(--muted-foreground); font-size: 11px; font-weight: 500; }
  .message p { padding: 11px 14px; border-radius: 14px; background: var(--muted); white-space: pre-wrap; line-height: 1.6; }
  .message.assistant p { padding: 0; background: transparent; }
  .conversation-runs { display: grid; gap: 8px; padding: 14px; border: 1px solid var(--border); border-radius: 10px; }
  .conversation-runs a { display: flex; align-items: center; gap: 8px; color: inherit; text-decoration: none; font-size: 12px; }
  .conversation-runs a:hover { text-decoration: underline; }
  .run-name { font-weight: 500; }
  .run-status { color: var(--muted-foreground); }
  .run-status.running { color: var(--status-running); }
  .run-status.error, .run-error { color: var(--destructive); }
  code { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: var(--font-mono); font-size: 11px; }
  .working-state, .error-state, .history-state { display: flex; align-items: center; gap: 8px; color: var(--muted-foreground); font-size: 12px; }
  .history-state, .error-state { flex-direction: column; align-items: flex-start; padding: 12px; border: 1px solid var(--border); border-radius: 8px; }
  .error-state { color: var(--destructive); }
  .empty-transcript, .no-selection { display: grid; place-items: center; align-content: center; gap: 10px; height: 100%; color: var(--muted-foreground); text-align: center; }
  .empty-transcript h3, .no-selection h2 { color: var(--foreground); font-size: 16px; font-weight: 500; }
  .empty-transcript p, .no-selection p { max-width: 460px; font-size: 13px; }
  .composer { position: relative; flex: none; width: min(720px, calc(100% - 48px)); margin: 0 auto 24px; }
  textarea { box-sizing: border-box; width: 100%; min-height: 116px; max-height: 40vh; resize: vertical; padding: 13px 48px 38px 14px; border: 1px solid var(--border); border-radius: 14px; outline: none; color: var(--foreground); background: var(--background); font: inherit; }
  textarea:focus { border-color: var(--ring); }
  .composer-meta { position: absolute; bottom: 15px; left: 14px; right: 52px; display: flex; align-items: center; gap: 12px; color: var(--muted-foreground); font-size: 11px; pointer-events: none; }
  .composer-meta code { flex: 1; text-align: right; }
  .send { position: absolute; right: 10px; bottom: 12px; display: grid; place-items: center; width: 30px; height: 30px; border: 0; border-radius: 8px; color: var(--primary-foreground); background: var(--primary); cursor: pointer; }
  .send:disabled { opacity: .4; cursor: default; }
  :global(.spin) { animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (max-width: 600px) { .composer-meta code { display: none; } }
</style>
