<script lang="ts">
 import ArrowUpIcon from '@lucide/svelte/icons/arrow-up';
 import { steer } from '../../../routes/steer.remote';
 import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
 import { errorMessage } from '#lib/errors';
 import { stopTurn } from '../../../routes/control.remote';
 import Pip from '../Pip.svelte';
 let { run, session, disabled = false, ended = false, reason = '', target, turn, ordinal }: { run: string; session: string; disabled?: boolean; ended?: boolean; reason?: string; target?: string; turn?: string; ordinal?: number } = $props();
 let stopping = $state(false);
 const instance = $props.id();
 const form = $derived(steer.for(`agent-steer-${run}-${session}-${instance}`));
 const message = $derived(form.fields.message.value() ?? '');
 const unavailable = $derived(disabled || ended);
 const explanation = $derived(reason || (ended ? 'This turn has ended. Steering is closed.' : 'Read at the next model call. Dropped if the turn ends first.'));
 let feedback = $state('');
</script>
<footer class="steer-box">
 {#if target}<p class="mode"><Pip state={ended ? 'ended' : 'running'} />{ended ? target : `Steer ${target}${ordinal ? ` · turn ${ordinal}` : ''}`}</p>{/if}
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
  {#if turn && !ended}<button class="stop" type="button" disabled={stopping} onclick={async () => { stopping = true; try { const result = await stopTurn({ run, turn }); feedback = result.accepted ? 'Stop accepted. Waiting for the turn to finish.' : 'This turn did not accept the stop request.'; } catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); } finally { stopping = false; } }}>Stop turn</button>{/if}
  {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p class="feedback" role="alert">{issue.message}</p>{/each}
  {#if feedback}<p class="feedback" role="status">{feedback}</p>{/if}
 </form>
</footer>
<style>
 .mode { display:flex; align-items:center; gap:8px; max-width:760px; margin:0 auto 10px; font:500 13px/18px var(--font-sans); } .stop { display:block; margin:8px 0 0 auto; padding:5px 10px; border:1px solid var(--border); border-radius:5px; background:var(--background); color:var(--foreground); font:12px var(--font-sans); cursor:pointer; }
 .steer-box { padding: 12px 20px 16px; flex: none; } form { max-width: 680px; margin: 0 auto; } .compose { position: relative; border: 1px solid var(--border); border-radius: 14px; padding: 11px 45px 11px 13px; background: var(--background); }
 textarea { display: block; width: 100%; box-sizing: border-box; resize: vertical; min-height: 38px; padding: 0; color: var(--foreground); background: transparent; font: 13px/1.6 var(--font-sans); border: 0; outline: 0; } textarea:disabled { opacity: .55; } .compose:focus-within { outline: 2px solid var(--ring); outline-offset: 2px; }
 .send { position: absolute; right: 10px; bottom: 10px; display: grid; place-items: center; width: 28px; height: 28px; border: 0; border-radius: 50%; color: var(--background); background: var(--foreground); cursor: pointer; } .send:disabled { opacity: .25; cursor: default; } .help { margin: 7px 0 0; color: var(--muted-foreground); font-size: 10px; line-height: 1.5; } .feedback { margin: 7px 0 0; color: var(--muted-foreground); font-size: 12px; line-height: 1.5; }
</style>
