<script lang="ts">
  import '@xyflow/svelte/dist/style.css';
  import '../style.css';
  let { children } = $props();
</script>
{@render children()}
