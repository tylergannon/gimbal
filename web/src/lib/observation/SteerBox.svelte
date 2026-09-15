<script lang="ts">
	import { steer } from '../../routes/steer.remote';

	// One box on one session that is mid-turn. A session with nothing running
	// gets no box: a steer sent to an idle session is dropped, so the page
	// does not offer the action. The caller decides that; this draws it.
	let { run, session, name }: { run: string; session: string; name: string } = $props();

	// for(session) isolates this instance from every other box on the page,
	// so a message sent to one agent does not show its answer under another.
	const box = $derived(steer.for(session));
	// What the server said, when it could not answer at all: the run ended,
	// or the session is no longer part of it. kit puts a field's own issues
	// on the field, but a rejected submission is ours to show.
	let refused = $state('');
</script>

<form
	class="steer"
	{...box.enhance(async ({ submit }) => {
		refused = '';
		try {
			await submit();
		} catch (error) {
			refused = error instanceof Error ? error.message : String(error);
		}
	})}
>
	<input {...box.fields.run.as('hidden', run)} />
	<input {...box.fields.session.as('hidden', session)} />
	<textarea {...box.fields.message.as('text')} rows="2" placeholder="Say something to {name}"></textarea>
	<button disabled={!!box.pending}>{box.pending ? 'Sending' : 'Send'}</button>
</form>
{#each box.fields.message.issues() ?? [] as issue (issue.message)}
	<p class="steer-said issue">{issue.message}</p>
{/each}
{#if refused}
	<p class="steer-said issue">{refused}</p>
{:else if box.result}
	<p class="steer-said">
		{box.result.landed
			? 'Sent into the running turn.'
			: 'Not sent: no turn was running to receive it, so the message was dropped.'}
	</p>
{/if}

<style>
	.steer { display: flex; align-items: flex-start; gap: .5rem; }
	.steer textarea {
		flex: 1; resize: vertical; font: inherit; padding: .4rem .5rem;
		border: 1px solid #d3d9e3; border-radius: .35rem;
	}
	.steer button {
		font: inherit; padding: .4rem .9rem; border: 1px solid #174d9b; border-radius: .35rem;
		background: #e3efff; color: #174d9b; cursor: pointer;
	}
	.steer button:disabled { cursor: default; opacity: .6; }
	.steer-said { margin: .35rem 0 0; color: #556070; font-size: .8rem; }
	.steer-said.issue { color: #a22525; }
</style>
