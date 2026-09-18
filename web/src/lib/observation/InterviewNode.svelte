<script lang="ts">
	import type { InterviewRow, RunObservation, RunStatus, SessionRow, TurnRow } from './index.js';
	import InterviewAnswerBox from './InterviewAnswerBox.svelte';
	import SessionTimeline from './SessionTimeline.svelte';
	import SteerBox from './SteerBox.svelte';

	let {
		run,
		status,
		scope,
		session,
		turns,
		questions,
		revision,
		observation
	}: {
		run: string;
		status: RunStatus;
		scope: string;
		session: SessionRow | undefined;
		turns: TurnRow[];
		questions: InterviewRow[];
		revision: number;
		observation: RunObservation;
	} = $props();

	const title = $derived(questions[0]?.name ?? 'interview');
	const sessionID = $derived(questions[0]?.session ?? turns[0]?.session ?? '');
</script>

<section class="interview" data-interview-node={`${scope}/${sessionID}`}>
	<header>
		<div>
			<p class="eyebrow">Interview · {title}</p>
			<h2>{session?.name ?? sessionID}</h2>
			<p>{session?.adapter ?? 'agent'} · {session?.model ?? 'model unavailable'}</p>
		</div>
		<span>{scope}</span>
	</header>

	<div class="turns">
		{#each turns as turn (turn.id)}
			{@const state = observation.state(turn.id)}
			<section class="turn" data-interview-turn={turn.id}>
				<header><span>{turn.id}</span><span>{turn.ended === 0 ? 'running' : 'ended'}</span></header>
				{#if state}<SessionTimeline {state} {revision} {observation} turn={turn.id} />{/if}
				{#if status === 'running' && turn.ended === 0}
					<SteerBox run={run} session={turn.session} name={session?.name ?? turn.session} />
				{/if}
			</section>
		{/each}
	</div>

	<div class="exchanges">
		{#each questions as question (question.question_id)}
			<article data-interview-question={question.question_id}>
				<p class="speaker">Question</p>
				<p class="prose">{question.question}</p>
				{#if question.status === 'answered'}
					<p class="speaker">Answer</p>
					<p class="prose">{question.answer.trim() === '' ? 'Interview ended by the person.' : question.answer}</p>
				{:else if status === 'running'}
					<InterviewAnswerBox {run} questionID={question.question_id} />
				{:else}
					<p class="unavailable">This question is no longer answerable because the run has ended.</p>
				{/if}
			</article>
		{/each}
	</div>
</section>

<style>
	.interview { margin-top: 2rem; display: grid; gap: 1rem; }
	.interview > header { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
	h2, p { margin: 0; }
	.eyebrow, .speaker { color: #697386; font-size: .75rem; text-transform: uppercase; letter-spacing: .08em; }
	.interview > header div > p:last-child, .interview > header > span { color: #697386; font-size: .85rem; }
	.turns, .exchanges { display: grid; gap: 1rem; }
	.turn { display: grid; gap: 1rem; border-left: 2px solid #dfe3ea; padding-left: 1rem; }
	.turn > header { display: flex; justify-content: space-between; gap: 1rem; color: #697386; font-size: .75rem; }
	article { display: grid; gap: .45rem; border: 1px solid #dfe3ea; border-radius: .65rem; padding: .85rem 1rem; background: #fff; }
	.prose { white-space: pre-wrap; }
	.unavailable { color: #697386; font-size: .8rem; }
</style>
