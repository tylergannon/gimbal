<script lang="ts">
 import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
 import { steer } from '../../../routes/steer.remote';
 import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
 import { errorMessage } from '#lib/errors';
 let { run, session, disabled = false, ended = false, reason = '' }: { run: string; session: string; disabled?: boolean; ended?: boolean; reason?: string } = $props();
 const instance = $props.id();
 const form = $derived(steer.for(`agent-steer-${run}-${session}-${instance}`));
 const message = $derived(form.fields.message.value() ?? '');
 const unavailable = $derived(disabled || ended);
 const explanation = $derived(reason || (ended ? 'Between turns. Steering is available while this agent is working.' : 'Read at the next model call. Dropped if the turn ends first.'));
 let feedback = $state('');
</script>
<footer class="steer-box">
 <form {...form.enhance(async (submitted) => {
   if (unavailable) return;
   feedback = '';
   try { await submitted.submit(); if (submitted.result) { feedback = submitted.result.landed ? 'Steer landed.' : 'The turn ended before this steer landed. It was dropped.'; submitted.element.reset(); } }
   catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); }
 })}>
  <input {...form.fields.run.as('hidden', run)} /><input {...form.fields.session.as('hidden', session)} />
  <div class="compose"><textarea {...form.fields.message.as('text')} rows={2} placeholder="Steer this turn…" aria-label="Steer this turn" disabled={unavailable || !!form.pending} onkeydown={(event) => { if (event.key === 'Enter' && !event.shiftKey && !event.isComposing) { event.preventDefault(); if (!unavailable && !form.pending && message.trim()) event.currentTarget.form?.requestSubmit(); } }}></textarea>
  <button type="submit" class="send" aria-label="Send steer" title={unavailable ? explanation : 'Send steer'} disabled={unavailable || !!form.pending || !message.trim()}><ArrowUpIcon size={17} /></button></div>
  <p class="help">{explanation}</p>
  {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p class="feedback" role="alert">{issue.message}</p>{/each}
  {#if feedback}<p class="feedback" role="status">{feedback}</p>{/if}
 </form>
</footer>
<style>
 .steer-box { padding: 12px 20px 16px; flex: none; } form { max-width: 680px; margin: 0 auto; } .compose { position: relative; border: 1px solid var(--border); border-radius: 14px; padding: 11px 45px 11px 13px; background: var(--background); }
 textarea { display: block; width: 100%; box-sizing: border-box; resize: vertical; min-height: 38px; padding: 0; color: var(--foreground); background: transparent; font: 13px/1.6 var(--font-sans); border: 0; outline: 0; } textarea:disabled { opacity: .55; } .compose:focus-within { outline: 2px solid var(--ring); outline-offset: 2px; }
 .send { position: absolute; right: 10px; bottom: 10px; display: grid; place-items: center; width: 28px; height: 28px; border: 0; border-radius: 50%; color: var(--background); background: var(--foreground); cursor: pointer; } .send:disabled { opacity: .25; cursor: default; } .help { margin: 7px 0 0; color: var(--muted-foreground); font-size: 10px; line-height: 1.5; } .feedback { margin: 7px 0 0; color: var(--muted-foreground); font-size: 12px; line-height: 1.5; }
</style>
