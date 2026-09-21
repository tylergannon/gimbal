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

<Story
	name="Assistant with tool calls"
	args={{
		revision: 0,
		message: {
			id: 'message-2',
			type: 'assistant',
			time: { completed: 1 },
			content: [
				{
					id: 'text-2',
					type: 'text',
					text: "I'll add pointer-based drag handling to the rack tiles first, then the board squares."
				},
				{
					id: 'reasoning-2',
					type: 'reasoning',
					text: 'Native HTML5 drag-and-drop is the simplest path; fall back to pointer events on touch.',
					time: { created: 0, completed: 10 }
				},
				{
					id: 'tool-read',
					type: 'tool',
					name: 'Read',
					time: { created: 0, ran: 20, completed: 220 },
					state: {
						status: 'completed',
						input: { file_path: 'web/src/lib/game.ts' },
						content: [{ type: 'text', text: 'export function place(tile, square) {\n\treturn { ...tile, square };\n}' }]
					}
				},
				{
					id: 'tool-edit',
					type: 'tool',
					name: 'Edit',
					time: { created: 0, ran: 20, completed: 420 },
					state: {
						status: 'completed',
						input: {
							file_path: 'web/src/routes/+page.svelte',
							old_string: 'let selected = $state<string | null>(null);',
							new_string: 'let selected = $state<string | null>(null);\n\tlet dragTile = $state<string | null>(null);'
						},
						content: [{ type: 'text', text: 'web/src/routes/+page.svelte has been updated successfully.' }]
					}
				},
				{
					id: 'tool-bash',
					type: 'tool',
					name: 'Bash',
					time: { created: Date.now() - 12_000 },
					state: {
						status: 'running',
						input: { command: 'vitest run --browser' },
						metadata: {}
					}
				}
			],
			tokens: {
				input: 8,
				output: 673,
				reasoning: 190,
				cache: { read: 48218, write: 1109 }
			},
			cost: 0.31
		}
	}}
/>
