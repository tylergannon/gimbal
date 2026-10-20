<script lang="ts">
  import type { InteractiveRow } from '#lib/observation';
  import { answerInteractive, finishInteractive } from '../../routes/interactive.remote';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  import StopTurnButton from './StopTurnButton.svelte';
  let { run, question, composer = false, target, turn }: { run: string; question: InteractiveRow; composer?: boolean; target?: string; turn?: string } = $props();
  const instance = $props.id();
  const form = $derived(answerInteractive.for(`run-question-${run}-${question.question_id}-${instance}`));
  let feedback = $state('');
 let requestId=$state(crypto.randomUUID());
 let finishing=$state(false);
 async function finish(){finishing=true;feedback='';try{await finishInteractive({run,address:question.address,id:crypto.randomUUID()});feedback='Finish accepted.';}catch(error){if(!navigateOnUnauthorized(error))feedback=errorMessage(error);}finally{finishing=false;}}
</script>
<section aria-label="Waiting question" class="question">
  <p class="label">{composer ? `Answer ${target || question.name}` : `Needs you · ${question.scope || 'root'}`}</p>
  {#if !composer}<h2>{question.question}</h2>{/if}
  <form {...form.enhance(async submitted => {
    feedback = '';
    try {
      await submitted.submit();
 if(submitted.result?.accepted)requestId=crypto.randomUUID();
      if (submitted.result) feedback = submitted.result.accepted ? 'Answer accepted.' : 'This question is no longer waiting.';
    } catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); }
  })}>
    <input {...form.fields.request_id.as('hidden',requestId)} />
    <input {...form.fields.run.as('hidden', run)} />
    <input {...form.fields.question_id.as('hidden', question.address)} />
    <textarea {...form.fields.answer.as('text')} aria-label="Answer the waiting question" placeholder="Answer the question above…" rows={2} required disabled={!!form.pending}></textarea>
    {#if turn}<StopTurnButton {run} {turn} onfeedback={message => feedback = message} />{/if}<button type="submit" disabled={!!form.pending}>{form.pending ? 'Sending…' : 'Send answer'}</button>
    {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p role="alert">{issue.message}</p>{/each}
    {#if feedback}<p role="status">{feedback}</p>{/if}
  </form>
 <button type="button" disabled={finishing} onclick={finish}>{finishing?'Finishing…':'Finish interaction'}</button>
</section>
<style>
  .question { padding: 12px 20px; border-bottom: 1px solid var(--border); }
  .label { margin: 0 0 4px; color: var(--muted-foreground); font-size: 11px; }
  h2 { margin: 0 0 8px; font-size: 13px; font-weight: 500; }
  form { display: flex; flex-wrap: wrap; align-items: end; gap: 8px; }
  textarea { flex: 1; min-width: 180px; padding: 6px 8px; font: 13px/1.5 var(--font-sans); color: var(--foreground); background: var(--background); border: 1px solid var(--border); border-radius: 6px; }
  button { padding: 7px 10px; font: inherit; font-size: 12px; border: 1px solid var(--border); border-radius: 6px; color: var(--foreground); background: var(--muted); cursor: pointer; }
  form p { flex-basis: 100%; margin: 0; font-size: 12px; }
</style>
