<script module lang="ts">
	import { defineMeta } from "@storybook/addon-svelte-csf";
	import Pip, { type RowStatus } from "./Pip.svelte";

	const { Story } = defineMeta({
		title: "Gimble/Run/Pip",
		component: Pip,
		tags: ["autodocs"],
		argTypes: {
			status: {
				control: "select",
				options: ["running", "completed", "failed", "cancelled", "ended", "pending", "answered"],
			},
			error: { control: "text" },
			size: { control: { type: "range", min: 6, max: 24, step: 1 } },
		},
		args: { status: "ended", error: "", size: 10 },
	});

	/** The legend of States.html: each shape, and a row that produces it. */
	const legend: { wording: string; status?: RowStatus; error?: string }[] = [
		{ wording: "ended", status: "ended" },
		{ wording: "running", status: "running" },
		{ wording: "failed", status: "ended", error: "exit status 1" },
		{ wording: "waiting for you", status: "pending" },
		{ wording: "not yet", status: undefined },
	];
</script>

<Story name="Ended" args={{ status: "ended" }} />

<Story name="Running" args={{ status: "running" }} />

<Story name="Failed" args={{ status: "failed" }} />

<Story name="Waiting for you" args={{ status: "pending" }} />

<Story name="Not yet" args={{ status: undefined }} />

<Story name="All five" asChild>
	<div class="legend">
		{#each legend as item (item.wording)}
			<span class="item"><Pip status={item.status} error={item.error} />{item.wording}</span>
		{/each}
	</div>
</Story>

<Story name="Sizes" asChild>
	<div class="legend">
		{#each [10, 8, 6] as size (size)}
			<span class="item"><Pip status="running" {size} />{size}px</span>
		{/each}
	</div>
</Story>

<style>
	.legend {
		display: flex;
		align-items: center;
		gap: 14px;
		font-size: 13px;
		line-height: 18px;
		color: var(--foreground);
	}

	.item {
		display: flex;
		align-items: center;
		gap: 6px;
	}
</style>
