<script module lang="ts">
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import ToolCall from './ToolCall.svelte';

	const readOutput = Array.from(
		{ length: 150 },
		(_, i) => `${(i + 1).toString().padStart(3, ' ')}\tconst tile = rack[${i}];`
	).join('\n');

	const readPart = {
		id: 'tool-read',
		type: 'tool',
		name: 'Read',
		time: { created: 0, ran: 20, completed: 220 },
		state: {
			status: 'completed',
			input: { file_path: 'web/src/lib/game.ts' },
			content: [{ type: 'text', text: readOutput }]
		}
	};

	const editPart = {
		id: 'tool-edit',
		type: 'tool',
		name: 'Edit',
		time: { created: 0, ran: 20, completed: 420 },
		state: {
			status: 'completed',
			input: {
				file_path: 'web/src/routes/+page.svelte',
				old_string: 'let selected = $state<string | null>(null);\n\tconst rows = tiles.map((t) => `<Tile value={t} />`);',
				new_string:
					'let selected = $state<string | null>(null);\n\tlet dragTile = $state<string | null>(null);\n\tconst rows = tiles.map((t) => `<Tile value={t} dragging={t === dragTile} />`);'
			},
			content: [{ type: 'text', text: 'web/src/routes/+page.svelte has been updated successfully.' }]
		}
	};

	const bashRunningPart = {
		id: 'tool-bash',
		type: 'tool',
		name: 'Bash',
		time: { created: Date.now() - 12_000 },
		state: {
			status: 'running',
			input: { command: 'vitest run --browser' },
			metadata: {}
		}
	};

	const errorPart = {
		id: 'tool-error',
		type: 'tool',
		name: 'Read',
		time: { created: 0, ran: 10, completed: 40 },
		state: {
			status: 'error',
			input: { file_path: 'web/src/lib/missing.ts' },
			error: "ENOENT: no such file or directory, open 'web/src/lib/missing.ts'"
		}
	};

	const unbreakablePart = {
		id: 'tool-unbreakable',
		type: 'tool',
		name: 'Bash',
		time: { created: 0, ran: 10, completed: 340 },
		state: {
			status: 'completed',
			input: { command: 'sha256sum dist/bundle.js' },
			content: [{ type: 'text', text: 'x'.repeat(600) }]
		}
	};

	const subagentEvents = Array.from({ length: 12 }, (_, i) => ({
		message: { content: [{ text: `Subagent step ${i + 1}: inspecting reconnaissance notes.` }] }
	}));

	const subagentPart = {
		id: 'tool-subagent',
		type: 'tool',
		name: 'Task',
		time: { created: 0, ran: 10, completed: 4200 },
		state: {
			status: 'completed',
			input: { description: 'Research the drag-and-drop implementation options' },
			content: [
				{ type: 'text', text: 'Recommend native HTML5 drag-and-drop with pointer fallback.' },
				{ type: 'transcript', events: subagentEvents }
			]
		}
	};

	const { Story } = defineMeta({
		title: 'Gimble/Run/Detail/Tool call',
		component: ToolCall,
		tags: ['autodocs']
	});
</script>

<Story name="Completed read, 150 lines" asChild>
	<div style="width: 448px;">
		<ToolCall part={readPart} />
	</div>
</Story>

<Story name="Completed read, 150 lines (900px)" asChild>
	<div style="width: 900px;">
		<ToolCall part={readPart} />
	</div>
</Story>

<Story name="Edit with tabs and markup" asChild>
	<div style="width: 448px;">
		<ToolCall part={editPart} />
	</div>
</Story>

<Story name="Edit with tabs and markup (900px)" asChild>
	<div style="width: 900px;">
		<ToolCall part={editPart} />
	</div>
</Story>

<Story name="Bash running, no output yet" asChild>
	<div style="width: 448px;">
		<ToolCall part={bashRunningPart} />
	</div>
</Story>

<Story name="Bash running, no output yet (900px)" asChild>
	<div style="width: 900px;">
		<ToolCall part={bashRunningPart} />
	</div>
</Story>

<Story name="Tool in error" asChild>
	<div style="width: 448px;">
		<ToolCall part={errorPart} />
	</div>
</Story>

<Story name="Tool in error (900px)" asChild>
	<div style="width: 900px;">
		<ToolCall part={errorPart} />
	</div>
</Story>

<Story name="Unbreakable 600-char token" asChild>
	<div style="width: 448px;">
		<ToolCall part={unbreakablePart} />
	</div>
</Story>

<Story name="Unbreakable 600-char token (900px)" asChild>
	<div style="width: 900px;">
		<ToolCall part={unbreakablePart} />
	</div>
</Story>

<Story name="Subagent transcript" asChild>
	<div style="width: 448px;">
		<ToolCall part={subagentPart} />
	</div>
</Story>

<Story name="Subagent transcript (900px)" asChild>
	<div style="width: 900px;">
		<ToolCall part={subagentPart} />
	</div>
</Story>
