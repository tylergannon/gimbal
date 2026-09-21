<script lang="ts">
	import CheckIcon from '@lucide/svelte/icons/check';
	import CopyIcon from '@lucide/svelte/icons/copy';
	import WrapTextIcon from '@lucide/svelte/icons/wrap-text';

	const TRUNCATE_AT = 4000;
	const PREVIEW_LEN = 2000;

	let {
		label,
		text,
		maxHeight = 260,
		wrap = true,
		tone,
		anchor,
		prose = false
	}: {
		label: string;
		text: string;
		maxHeight?: number;
		wrap?: boolean;
		tone?: 'error';
		anchor?: 'top' | 'bottom';
		/** Prose (a written result, not data): sans font at 14px instead of
		 * mono, with the text's own newlines shown as real line breaks. JSON
		 * and other structured payloads stay mono and pretty-printed. */
		prose?: boolean;
	} = $props();

	// Seeded once from the initial prop: this is a local toggle from here on,
	// not a mirror of a prop that can change out from under the reader.
	let wrapped = $state(wrap);
	let expanded = $state(false);
	let copied = $state(false);
	let scroller: HTMLDivElement | undefined = $state();

	const truncated = $derived(text.length > TRUNCATE_AT);
	const shown = $derived(truncated && !expanded ? text.slice(0, PREVIEW_LEN) : text);

	const formatSize = (bytes: number): string => {
		if (bytes < 1024) return `${bytes} B`;
		const kb = bytes / 1024;
		return `${kb < 10 ? kb.toFixed(1) : Math.round(kb)} KB`;
	};

	$effect(() => {
		// Anchor to the tail once, when the block first mounts with content.
		shown;
		if (anchor === 'bottom' && scroller) scroller.scrollTop = scroller.scrollHeight;
	});

	async function copy() {
		try {
			await navigator.clipboard.writeText(text);
			copied = true;
			setTimeout(() => (copied = false), 1200);
		} catch {
			// Clipboard access denied or unavailable; nothing more to do.
		}
	}
</script>

<div class="payload" class:error={tone === 'error'}>
	<div class="strip">
		<span class="label">{label}</span>
		<span class="size">{formatSize(text.length)}</span>
		<span class="spacer"></span>
		<button type="button" class="tool-btn" onclick={copy}>
			{#if copied}<CheckIcon size={13} />Copied{:else}<CopyIcon size={13} />Copy{/if}
		</button>
		<button
			type="button"
			class="tool-btn"
			class:active={wrapped}
			aria-pressed={wrapped}
			onclick={() => (wrapped = !wrapped)}
		>
			<WrapTextIcon size={13} />Wrap
		</button>
	</div>
	<div
		class="scroller"
		class:wrap={wrapped}
		class:prose
		style={`max-height: ${maxHeight}px`}
		bind:this={scroller}
	>
		<pre>{shown}</pre>
	</div>
	{#if truncated && !expanded}
		<button type="button" class="show-all" onclick={() => (expanded = true)}>
			Show all ({formatSize(text.length)})
		</button>
	{/if}
</div>

<style>
	.payload {
		display: flex;
		min-width: 0;
		flex-direction: column;
		background: var(--code);
		border: 1px solid var(--map-line);
		border-radius: 6px;
		overflow: hidden;
	}
	.payload.error {
		border-color: var(--destructive);
	}
	.strip {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		padding: 4px 8px;
		color: var(--status-muted);
		font-size: 13px;
		background: var(--surface);
		border-bottom: 1px solid var(--map-line);
	}
	.label {
		color: var(--foreground);
		font-weight: 600;
	}
	.size {
		font-family: var(--font-mono);
	}
	.spacer {
		flex: 1;
	}
	.tool-btn {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 6px;
		color: var(--status-muted);
		font: inherit;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 4px;
		cursor: pointer;
	}
	.tool-btn:hover {
		color: var(--foreground);
		border-color: var(--map-line);
	}
	.tool-btn.active {
		color: var(--foreground);
		background: var(--background);
		border-color: var(--map-line-strong);
	}
	.scroller {
		min-width: 0;
		overflow-x: hidden;
		overflow-y: auto;
	}
	.scroller:not(.wrap) {
		overflow-x: auto;
	}
	.scroller.wrap pre {
		overflow-wrap: anywhere;
		white-space: pre-wrap;
	}
	.scroller:not(.wrap) pre {
		white-space: pre;
	}
	pre {
		min-width: 0;
		margin: 0;
		padding: 8px 10px;
		color: var(--code-foreground);
		font-family: var(--font-mono);
		font-size: 13px;
		line-height: 1.5;
	}
	.scroller.prose pre {
		font-family: var(--font-sans);
		font-size: 14px;
		line-height: 1.5;
	}
	.payload.error pre {
		color: var(--destructive);
	}
	.show-all {
		padding: 6px 8px;
		color: var(--status-live);
		font: inherit;
		font-size: 13px;
		text-align: left;
		background: var(--surface);
		border: none;
		border-top: 1px solid var(--map-line);
		cursor: pointer;
	}
	.show-all:hover {
		color: var(--foreground);
	}
</style>
