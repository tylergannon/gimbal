<script module lang="ts">
	import type { InterviewRow, RunRow, ScopeRow } from "../observation/index.js";

	/** Every status the run's own rows carry. A step the run has not reached
	 * has no row at all, which is how this component hears "not yet". */
	export type RowStatus = RunRow["status"] | ScopeRow["status"] | InterviewRow["status"];

	/** The five shapes of States.html, shape before colour. */
	type Shape = "ended" | "running" | "failed" | "waiting" | "not-yet";

	/** What each shape is called, for anyone who cannot see it. */
	const wording: Record<Shape, string> = {
		ended: "ended",
		running: "running",
		failed: "failed",
		waiting: "waiting for you",
		"not-yet": "not yet",
	};
</script>

<script lang="ts">
	type Props = {
		/** The status of the row this pip stands for, absent when the run has
		 * written no row for it yet. */
		status?: RowStatus;
		/** That row's `error`. A row that ended carrying one failed. */
		error?: string;
		/** Diameter in px: 10 on the map, 8 in a badge, 6 in a chip. */
		size?: number;
	};

	const { status, error = "", size = 10 }: Props = $props();

	// A row's status is what it ended as; its error is whether that was a
	// failure. A cancelled run stopped rather than failed, so it reads as
	// ended unless it carries an error of its own.
	const shape = $derived.by((): Shape => {
		if (status === undefined) return "not-yet";
		if (status === "failed" || error !== "") return "failed";
		if (status === "running") return "running";
		if (status === "pending") return "waiting";
		return "ended";
	});
</script>

<span
	class="pip {shape}"
	data-shape={shape}
	role="img"
	aria-label={wording[shape]}
	style="--pip-size: {size}px"
></span>

<style>
	.pip {
		position: relative;
		display: inline-block;
		width: var(--pip-size);
		height: var(--pip-size);
		border-radius: 9999px;
		flex-shrink: 0;
		box-sizing: border-box;
	}

	.ended {
		background: var(--foreground);
	}

	.running {
		border: 2px solid var(--live-ring);
		border-top-color: transparent;
		animation: spin 1s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.failed {
		background: var(--destructive);
	}

	/* The two strokes of the cross, cut out of the disc in the ground colour. */
	.failed::before,
	.failed::after {
		content: "";
		position: absolute;
		left: 2px;
		right: 2px;
		top: 4px;
		height: 1.5px;
		background: var(--background);
		transform: rotate(45deg);
	}

	.failed::after {
		transform: rotate(-45deg);
	}

	.waiting {
		background: var(--background);
		box-shadow: inset 0 0 0 2px var(--live);
	}

	.waiting::after {
		content: "";
		position: absolute;
		left: 3px;
		top: 3px;
		width: 4px;
		height: 4px;
		border-radius: 9999px;
		background: var(--live);
	}

	.not-yet {
		border: 1.5px dashed var(--ink-2);
	}
</style>
