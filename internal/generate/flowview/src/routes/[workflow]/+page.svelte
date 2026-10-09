<script lang="ts">
  import LivePreview from '../../LivePreview.svelte';
  import type { PageProps } from './$types';
  let {data}:PageProps=$props();
</script>
{#key data.name}<LivePreview name={data.name} initial={data.snapshot}/>{/key}
