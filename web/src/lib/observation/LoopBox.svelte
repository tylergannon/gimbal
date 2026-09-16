<script lang="ts">
	import { steerLoop } from '../../routes/steer.remote';

	// The controls on a loop's card while its dispatch is open: a message for
	// its planner, and the wrap-up instruction as a button of its own. Unlike
	// a steer to a session, nothing here needs a turn to be running: a
	// planner is not always in one, so the message waits for its next
	// decision. The caller decides whether the loop is still dispatching;
	// this draws it.
	let { run, scope, name }: { run: string; scope: string; name: string } = $props();

	// for(...) isolates each form from every other on the page, so an answer
	// shows under the loop it belongs to, and the two forms of one loop do
	// not share a pending state.
	const box = $derived(steerLoop.for(scope));
	const wrap = $derived(steerLoop.for(`${scope} wrap up`));
	// What the server said when it could not answer at all: the run ended, or
	// the loop stopped dispatching. A field's own issues are kit's to place.
	let refused = $state('');
	const said = (error: unknown) => {
		refused = error instanceof Error ? error.message : String(error);
	};
</script>

<form
	class="loop"
	{...box.enhance(async (form) => {
		refused = '';
		try {
			await form.submit();
			// Empty the box only once the message is with the loop; kit's own
			// reset keeps the hidden fields.
			if (form.result) form.element.reset();
		} catch (error) {
			said(error);
		}
	})}
>
	<input {...box.fields.run.as('hidden', run)} />
	<input {...box.fields.scope.as('hidden', scope)} />
	<input {...box.fields.wrap_up.as('hidden', false)} />
	<textarea {...box.fields.message.as('text')} rows="2" placeholder="Say something to the planner of {name}"></textarea>
	<button disabled={!!box.pending}>{box.pending ? 'Sending' : 'Send'}</button>
</form>
<form
	class="loop"
	{...wrap.enhance(async (form) => {
		refused = '';
		try {
			await form.submit();
		} catch (error) {
			said(error);
		}
	})}
>
	<input {...wrap.fields.run.as('hidden', run)} />
	<input {...wrap.fields.scope.as('hidden', scope)} />
	<input {...wrap.fields.wrap_up.as('hidden', true)} />
	<button disabled={!!wrap.pending}>{wrap.pending ? 'Sending' : 'Wrap up'}</button>
</form>
{#each box.fields.message.issues() ?? [] as issue (issue.message)}
	<p class="loop-said issue">{issue.message}</p>
{/each}
{#if refused}
	<p class="loop-said issue">{refused}</p>
{:else if wrap.result}
	<p class="loop-said">Waiting for the planner's next decision: {wrap.result.message}</p>
{:else if box.result}
	<p class="loop-said">Waiting for the planner's next decision: {box.result.message}</p>
{/if}

<style>
	.loop { display: flex; align-items: flex-start; gap: .5rem; margin-top: .5rem; }
	.loop textarea {
		flex: 1; resize: vertical; font: inherit; padding: .4rem .5rem;
		border: 1px solid #d3d9e3; border-radius: .35rem;
	}
	.loop button {
		font: inherit; padding: .4rem .9rem; border: 1px solid #174d9b; border-radius: .35rem;
		background: #e3efff; color: #174d9b; cursor: pointer;
	}
	.loop button:disabled { cursor: default; opacity: .6; }
	.loop-said { margin: .35rem 0 0; color: #556070; font-size: .8rem; }
	.loop-said.issue { color: #a22525; }
</style>
