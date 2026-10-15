<script lang="ts">
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import { MessageSquare, Ellipsis, ExternalLink, RefreshCw, PanelTop } from '@lucide/svelte';
  import * as Popover from '#lib/components/ui/popover';
  import { viewFile } from '../workspace.remote';
  let { project, id, ondiscuss, onmove }: {
    project: string; id: string;
    ondiscuss?: (file: { id: string; name: string }) => void;
    onmove?: () => void;
  } = $props();
  const query = $derived(viewFile({ project, id }));
  const view = $derived(query.current);
  $effect(() => { if (query.error) navigateOnUnauthorized(query.error); });
  let menu = $state(false);
  let refreshing = $state(false);
  let failure = $state('');
  async function refresh() {
    refreshing = true; failure = '';
    try { await query.refresh(); }
    catch (error) { if (!navigateOnUnauthorized(error)) failure = errorMessage(error); }
    finally { refreshing = false; }
  }
</script>

{#if view}
<section class="viewer" aria-label={`File: ${view.file.name}`}>
  <header>
    <p class="path" title={`Files / ${view.file.name}`}><span>Files</span><span aria-hidden="true">/</span><strong>{view.file.name}</strong></p>
    <a class="discuss" href={`/projects/${project}/files/${id}/discuss`} onclick={(event) => { if (ondiscuss) { event.preventDefault(); ondiscuss(view.file); } }}><MessageSquare size={14} aria-hidden="true" />Discuss in chat</a>
    <Popover.Root bind:open={menu}>
      <Popover.Trigger class="file-more" aria-label="File actions"><Ellipsis size={18} aria-hidden="true" /></Popover.Trigger>
      <Popover.Content align="end" class="file-actions w-56 gap-1 p-1">
        <a class="file-action" href={view.url} target="_blank" rel="noreferrer" onclick={() => (menu = false)}><ExternalLink size={14} aria-hidden="true" />Open / download</a>
        <button class="file-action" disabled={refreshing} onclick={refresh}><RefreshCw size={14} aria-hidden="true" />{refreshing ? 'Refreshing…' : 'Refresh file'}</button>
        {#if onmove}<button class="file-action" onclick={() => { menu = false; onmove(); }}><PanelTop size={14} aria-hidden="true" />Move tab to new window</button>{/if}
      </Popover.Content>
    </Popover.Root>
  </header>
  {#if failure}<p class="notice" role="alert">Could not refresh this file: {failure} <button disabled={refreshing} onclick={refresh}>Retry</button></p>{/if}
  {#if view.file.mediaType.startsWith('image/')}
    <div class="image"><img src={view.url} alt={view.file.name} onerror={() => { failure = 'The image could not be loaded. Refresh the file to try again.'; }} /></div>
  {:else if view.html}<div class="reading"><article>{@html view.html}</article></div>
  {:else if view.file.mediaType.startsWith('text/')}<div class="reading"><pre>{view.text}</pre></div>
  {:else if view.file.mediaType === 'application/pdf'}<iframe title={view.file.name} src={view.url}></iframe>
  {:else}<div class="unsupported"><p>This file is available to download.</p><a href={view.url} target="_blank" rel="noreferrer">Open / download {view.file.name}</a></div>{/if}
</section>
{:else}
  <section class="viewer" aria-label="Document viewer"><p class="notice" role={query.error ? 'alert' : 'status'}>{query.error ? `Could not open this file: ${errorMessage(query.error)}` : 'Loading file…'}{#if query.error} <button disabled={refreshing} onclick={refresh}>Retry</button>{/if}</p></section>
{/if}

<style>
  .viewer { display: flex; flex-direction: column; height: 100%; min-height: 0; min-width: 0; color: var(--foreground); }
  header { display: flex; flex: none; align-items: center; gap: 12px; padding: 10px 24px; font-size: 12px; }
  .path { display: flex; align-items: baseline; flex: 1; min-width: 0; gap: 6px; margin: 0; color: var(--muted-foreground); }
  .path strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--foreground); font-weight: 500; }
  .discuss { display: inline-flex; align-items: center; flex: none; gap: 6px; padding: 6px 4px; color: var(--muted-foreground); text-decoration: none; white-space: nowrap; }
  .discuss:hover { color: var(--foreground); }
  header :global(.file-more) { display: grid; place-items: center; flex: none; width: 28px; height: 28px; padding: 0; border: 0; border-radius: 6px; color: var(--muted-foreground); background: none; cursor: pointer; }
  header :global(.file-more:hover), header :global(.file-more[data-state='open']) { background: var(--muted); color: var(--foreground); }
  :global(.file-actions .file-action) { display: flex; align-items: center; gap: 8px; min-height: 32px; padding: 6px 8px; border: 0; border-radius: 5px; color: inherit; background: none; font: inherit; font-size: 12px; text-align: left; text-decoration: none; cursor: pointer; }
  :global(.file-actions .file-action:hover) { background: var(--muted); }
  .reading { flex: 1; min-height: 0; overflow: auto; padding: 12px 24px 40px; }
  article, pre { max-width: 680px; margin: 0 auto; font-size: 15px; line-height: 1.625; overflow-wrap: anywhere; }
  pre { white-space: pre-wrap; tab-size: 4; font-family: var(--font-mono); }
  article :global(h1) { margin: 0 0 20px; font-size: 28px; font-weight: 600; line-height: 1.3; }
  article :global(h2) { margin: 28px 0 12px; font-size: 20px; font-weight: 600; line-height: 1.35; }
  article :global(h3), article :global(h4) { margin: 24px 0 10px; font-size: 16px; font-weight: 600; }
  article :global(p), article :global(ul), article :global(ol), article :global(blockquote), article :global(pre) { margin: 0 0 18px; }
  article :global(ul) { padding-left: 24px; list-style: disc; }
  article :global(ol) { padding-left: 24px; list-style: decimal; }
  article :global(li + li) { margin-top: 6px; }
  article :global(a) { color: inherit; text-decoration: underline; text-underline-offset: 3px; }
  article :global(blockquote) { padding-left: 16px; border-left: 2px solid var(--border); color: var(--muted-foreground); }
  article :global(pre) { max-width: 100%; overflow: auto; padding: 14px 16px; border-radius: 8px; background: var(--muted); white-space: pre; font: 13px/1.6 var(--font-mono); }
  article :global(code) { font-family: var(--font-mono); font-size: .875em; }
  article :global(img) { max-width: 100%; height: auto; }
  .image { display: grid; flex: 1; min-height: 0; place-items: center; overflow: auto; padding: 24px; }
  img { max-width: 100%; max-height: 100%; object-fit: contain; }
  iframe { flex: 1; min-height: 0; width: 100%; border: 0; }
  .notice { margin: 0; padding: 8px 24px; color: var(--destructive); font-size: 12px; }
  .notice button { border: 0; color: inherit; background: none; font: inherit; text-decoration: underline; cursor: pointer; }
  .unsupported { display: grid; align-content: start; justify-content: center; gap: 12px; padding: 48px 24px; color: var(--muted-foreground); font-size: 13px; }
  .unsupported p { margin: 0; } .unsupported a { color: var(--foreground); }
  @media (max-width: 600px) { header { gap: 8px; padding-inline: 16px; } .reading { padding-inline: 16px; } .path > span { display: none; } }
</style>
