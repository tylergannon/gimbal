<script lang="ts">
	import { AlertDialog as AlertDialogPrimitive } from "bits-ui";
	import { cn } from "#lib/utils.js";

	let {
		ref = $bindable(null),
		class: className,
		animate = true,
		...restProps
	}: AlertDialogPrimitive.OverlayProps & { animate?: boolean } = $props();
</script>

<AlertDialogPrimitive.Overlay
	bind:ref
	data-slot="alert-dialog-overlay"
	class={cn(animate && "data-open:animate-in data-closed:animate-out data-closed:fade-out-0 data-open:fade-in-0", "bg-black/10 duration-100 supports-backdrop-filter:backdrop-blur-xs fixed inset-0 z-50", className)}
	{...restProps}
/>
