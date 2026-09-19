<script module lang="ts">
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import MessageRow from './MessageRow.svelte';

	const { Story } = defineMeta({
		title: 'Gimble/Run/Message row',
		component: MessageRow,
		tags: ['autodocs']
	});
</script>

<Story
	name="Assistant response"
	args={{
		revision: 0,
		message: {
			id: 'message-1',
			type: 'assistant',
			time: { completed: 1 },
			content: [
				{
					id: 'text-1',
					type: 'text',
					text: 'The workflow finished and the generated application is ready.'
				}
			],
			tokens: {
				input: 128,
				output: 24,
				reasoning: 8,
				cache: { read: 32, write: 0 }
			},
			cost: 0.0042
		}
	}}
/>
