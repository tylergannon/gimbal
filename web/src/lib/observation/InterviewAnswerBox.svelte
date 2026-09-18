<script lang="ts">
	import { answerInterview } from '../../routes/interview.remote';

	let { run, questionID }: { run: string; questionID: string } = $props();

	// A form instance belongs to one opaque question ID. Concurrent interviews
	// therefore keep separate pending, result, and field state in the browser.
	const box = $derived(answerInterview.for(questionID));
	let refused = $state('');
</script>

<form
	class="answer"
	{...box.enhance(async (form) => {
		refused = '';
		try {
			await form.submit();
		} catch (error) {
			refused = error instanceof Error ? error.message : String(error);
		}
	})}
>
	<input {...box.fields.run.as('hidden', run)} />
	<input {...box.fields.question_id.as('hidden', questionID)} />
	<textarea {...box.fields.answer.as('text')} rows="3" placeholder="Type your answer"></textarea>
	<button disabled={!!box.pending || box.result?.accepted}>
		{box.pending ? 'Sending' : box.result?.accepted ? 'Answered' : 'Answer'}
	</button>
</form>
<p class="answer-help">Submit an empty answer to end the interview.</p>
{#if refused}<p class="answer-said issue">{refused}</p>{/if}

<style>
	.answer { display: flex; align-items: flex-start; gap: .5rem; }
	.answer textarea {
		flex: 1; resize: vertical; font: inherit; padding: .4rem .5rem;
		border: 1px solid #d3d9e3; border-radius: .35rem;
	}
	.answer button {
		font: inherit; padding: .4rem .9rem; border: 1px solid #174d9b; border-radius: .35rem;
		background: #e3efff; color: #174d9b; cursor: pointer;
	}
	.answer button:disabled { cursor: default; opacity: .6; }
	.answer-help, .answer-said { margin: .35rem 0 0; color: #697386; font-size: .8rem; }
	.answer-said.issue { color: #a22525; }
</style>
