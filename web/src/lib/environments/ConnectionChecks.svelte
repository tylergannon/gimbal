<script lang="ts">
  import Check from '@lucide/svelte/icons/check';
  import type { ConnectionProgress } from '#lib/skgo/environments/types';
  import StateGlyph from './StateGlyph.svelte';
  let { progress }: { progress?: ConnectionProgress } = $props();
</script>

{#if progress?.checks.length}
  <ol class="checks" aria-label="Connection checks" aria-live="polite">
    {#each progress.checks as check (check.id)}
      <li class={{ failed: check.fatal }}>
        <div class="check-heading">
          {#if check.state === 'completed'}<Check size={13} aria-hidden="true" />{:else}<StateGlyph state={check.state} />{/if}
          <strong>{check.label}</strong><span>{check.state === 'pending' ? 'Checking…' : check.state === 'completed' ? 'Passed' : check.fatal ? 'Cannot connect' : 'Needs attention'}</span>
        </div>
        {#if check.cause}<p>{check.cause}</p>{/if}
        {#if check.consequence}<p class="consequence">{check.consequence}</p>{/if}
        {#if check.fix}<p class="fix">{check.fix}</p>{/if}
      </li>
    {/each}
  </ol>
{/if}

<style>
  .checks { display: grid; gap: 14px; margin: 4px 0; padding: 14px 0; border-block: 1px solid var(--border); list-style: none; font-size: 12px; }
  li { display: grid; gap: 4px; }
  .check-heading { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
  strong { font-weight: 500; }
  .check-heading > span:last-child { margin-left: auto; color: var(--muted-foreground); font-size: 11px; }
  .failed .check-heading { color: var(--destructive); }
  p { margin: 0 0 0 21px; color: var(--muted-foreground); font-size: 12px; line-height: 1.5; overflow-wrap: anywhere; }
  .consequence { color: var(--foreground); }
  .fix { font-weight: 500; }
</style>
