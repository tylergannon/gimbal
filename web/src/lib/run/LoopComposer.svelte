<script lang="ts">
  import { steerLoop } from '../../routes/steer.remote';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import Pip from './Pip.svelte';
  let { run, scope, name }: { run: string; scope: string; name: string } = $props();
  const form = $derived(steerLoop.for(`transcript-loop-${run}-${scope}`));
  const message = $derived(form.fields.message.value() ?? '');
  let feedback = $state('');
</script>
<footer class="loop-compose">
  <p class="mode"><Pip state="running" />Steer {name}<span>· lands at the planner’s next decision</span></p>
  <form {...form.enhance(async submitted => { feedback = ''; try { await submitted.submit(); if (submitted.result) { feedback = 'Message queued for the planner’s next decision.'; submitted.element.reset(); } } catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); } })}>
    <input {...form.fields.run.as('hidden', run)} /><input {...form.fields.scope.as('hidden', scope)} /><input {...form.fields.wrap_up.as('hidden', false)} />
    <textarea {...form.fields.message.as('text')} aria-label="Steer the loop planner" placeholder="Say something to the planner…" rows={2}></textarea>
    <div class="actions"><span>The loop keeps this message until the planner reads it.</span><button disabled={!!form.pending || !message.trim()}>Send</button></div>
    {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p role="alert">{issue.message}</p>{/each}{#if feedback}<p role="status">{feedback}</p>{/if}
  </form>
</footer>
<style>
  .loop-compose { padding-top:12px; } .mode { display:flex; align-items:center; flex-wrap:wrap; gap:8px; margin:0 0 10px; font:500 13px/18px var(--font-sans); } .mode span, .actions span { color:var(--muted-foreground); font-weight:400; } textarea { display:block; width:100%; box-sizing:border-box; resize:vertical; padding:12px; color:var(--foreground); background:var(--background); border:1px solid var(--border); border-radius:10px; font:13px/20px var(--font-sans); } .actions { display:flex; align-items:center; justify-content:space-between; gap:12px; margin-top:8px; font-size:11px; } button { padding:6px 12px; border:1px solid var(--border); border-radius:5px; background:var(--foreground); color:var(--background); font:12px var(--font-sans); cursor:pointer; } button:disabled { opacity:.4; } form p { font-size:12px; }
</style>
