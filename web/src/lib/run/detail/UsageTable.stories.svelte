<script module lang="ts">
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import UsageTable from './UsageTable.svelte';

	const usage = {
		input: 8,
		output: 673,
		reasoning: 190,
		cache_read: 48218,
		cache_write: 1109,
		stated_cost: 0.31
	};

	const sessionUsage = {
		input: 12482,
		output: 1486,
		reasoning: 412,
		cache_read: 96436,
		cache_write: 2218,
		stated_cost: 0.62
	};

	const { Story } = defineMeta({
		title: 'Gimble/Run/Detail/Usage table',
		component: UsageTable,
		tags: ['autodocs']
	});
</script>

<Story name="This turn" asChild>
	<div style="width: 448px;">
		<UsageTable {usage} title="Usage · this turn" />
	</div>
</Story>

<Story name="This turn (900px)" asChild>
	<div style="width: 900px;">
		<UsageTable {usage} title="Usage · this turn" />
	</div>
</Story>

<Story name="Session total, no title" asChild>
	<div style="width: 448px;">
		<UsageTable usage={sessionUsage} />
	</div>
</Story>
