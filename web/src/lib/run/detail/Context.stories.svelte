<script module lang="ts">
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import type { TurnRow } from '../../observation/index.svelte.js';
	import { issue325GluedPrompt, issue325Snapshot } from '../fixtures/issue-325.js';
	import Payload from './Payload.svelte';
	import PromptAndContext from './PromptAndContext.svelte';
	import ScopeContext from './ScopeContext.svelte';

	const recordedTurn = issue325Snapshot.turns['coding.1/turn.2'] ;

	// The same turn as a run saved before turns recorded their context: one
	// glued prompt, and the context can only be inferred from the scopes.
	const inferredTurn: TurnRow = {
		...recordedTurn,
		context: undefined,
		prompt: issue325GluedPrompt
	};

	const { Story } = defineMeta({
		title: 'Gimble/Run/Detail/Prompt and context',
		tags: ['autodocs']
	});
</script>

<Story name="Prompt and context · recorded" asChild>
	<div style="width: 448px;">
		<PromptAndContext turn={recordedTurn} snapshot={issue325Snapshot} onscope={() => {}} />
	</div>
</Story>

<Story name="Prompt and context · recorded (900px)" asChild>
	<div style="width: 900px;">
		<PromptAndContext turn={recordedTurn} snapshot={issue325Snapshot} onscope={() => {}} />
	</div>
</Story>

<Story name="Prompt and context · inferred" asChild>
	<div style="width: 448px;">
		<PromptAndContext turn={inferredTurn} snapshot={issue325Snapshot} onscope={() => {}} />
	</div>
</Story>

<Story name="Scope context · task.1" asChild>
	<div style="width: 448px;">
		<ScopeContext
			snapshot={issue325Snapshot}
			scopeKey="implementation.1/task.1"
			onscope={() => {}}
		/>
	</div>
</Story>

<Story name="Scope context · loop" asChild>
	<div style="width: 448px;">
		<ScopeContext snapshot={issue325Snapshot} scopeKey="implementation.1" onscope={() => {}} />
	</div>
</Story>

<Story name="Scope context · empty" asChild>
	<div style="width: 448px;">
		<ScopeContext snapshot={issue325Snapshot} scopeKey="" onscope={() => {}} />
	</div>
</Story>

<Story name="Before · glued prompt" asChild>
	<div style="width: 448px;">
		<Payload label="prompt" text={issue325GluedPrompt} maxHeight={420} />
	</div>
</Story>
