<script module lang="ts">
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import Payload from './Payload.svelte';

	const shortText = 'web/src/routes/+page.svelte has been updated successfully.';
	const multilineText = [
		'let selected = $state<string | null>(null);',
		'let dragTile = $state<string | null>(null);',
		'',
		'function onDrop(tile: string, square: number) {',
		'\tboard[square] = tile;',
		'\tselected = null;',
		'}'
	].join('\n');
	const longText = Array.from(
		{ length: 400 },
		(_, i) => `line ${i + 1}: reading web/src/lib/game.ts to trace tile placement`
	).join('\n');
	const unbreakable = 'x'.repeat(600);

	const { Story } = defineMeta({
		title: 'Gimble/Run/Detail/Payload',
		component: Payload,
		tags: ['autodocs']
	});
</script>

<Story name="Short output" asChild>
	<div style="width: 448px;">
		<Payload label="output" text={shortText} />
	</div>
</Story>

<Story name="Short output (900px)" asChild>
	<div style="width: 900px;">
		<Payload label="output" text={shortText} />
	</div>
</Story>

<Story name="Multiline" asChild>
	<div style="width: 448px;">
		<Payload label="input" text={multilineText} />
	</div>
</Story>

<Story name="Multiline (900px)" asChild>
	<div style="width: 900px;">
		<Payload label="input" text={multilineText} />
	</div>
</Story>

<Story name="Show all threshold" asChild>
	<div style="width: 448px;">
		<Payload label="output" text={longText} />
	</div>
</Story>

<Story name="Error" asChild>
	<div style="width: 448px;">
		<Payload label="error" tone="error" text="ENOENT: no such file or directory, open 'web/src/lib/missing.ts'" />
	</div>
</Story>

<Story name="Unbreakable token" asChild>
	<div style="width: 448px;">
		<Payload label="output" text={unbreakable} />
	</div>
</Story>

<Story name="Anchored to bottom" asChild>
	<div style="width: 448px;">
		<Payload label="output" text={longText} anchor="bottom" />
	</div>
</Story>
