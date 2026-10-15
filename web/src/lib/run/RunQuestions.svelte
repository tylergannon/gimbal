<script lang="ts">
  import type { InterviewRow } from '#lib/observation';
  import { answerInterview } from '../../routes/interview.remote';
  import { navigateOnUnauthorized } from '#lib/auth/unauthorized';
  import { errorMessage } from '#lib/errors';
  let { run, question }: { run: string; question: InterviewRow } = $props();
  const form = $derived(answerInterview.for(`run-question-${run}-${question.question_id}`));
  let feedback = $state('');
</script>
<section aria-label="Waiting question" class="question">
  <p class="label">Needs you · {question.scope || 'root'}</p>
  <h2>{question.question}</h2>
  <form {...form.enhance(async submitted => {
    feedback = '';
    try {
      await submitted.submit();
      if (submitted.result) feedback = submitted.result.accepted ? 'Answer accepted.' : 'This question is no longer waiting.';
    } catch (error) { if (!navigateOnUnauthorized(error)) feedback = errorMessage(error); }
  })}>
    <input {...form.fields.run.as('hidden', run)} />
    <input {...form.fields.question_id.as('hidden', question.question_id)} />
    <textarea {...form.fields.answer.as('text')} aria-label="Answer the waiting question" rows={2} required disabled={!!form.pending}></textarea>
    <button type="submit" disabled={!!form.pending}>{form.pending ? 'Sending…' : 'Send answer'}</button>
    {#each form.fields.allIssues() ?? [] as issue (issue.message)}<p role="alert">{issue.message}</p>{/each}
    {#if feedback}<p role="status">{feedback}</p>{/if}
  </form>
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
