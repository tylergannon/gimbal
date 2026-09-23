<script lang="ts">
  import Conversations from '#lib/conversation/Conversations.svelte';
  let { data } = $props();
</script>

<Conversations {data} />
