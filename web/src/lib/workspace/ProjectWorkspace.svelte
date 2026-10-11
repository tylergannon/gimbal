<script lang="ts">
  import RecordingHistory from "./RecordingHistory.svelte";
  import RecordingTranscript from "./RecordingTranscript.svelte";
  import FileLibrary from "./FileLibrary.svelte";
  let { project, id = "" }: { project: string; id?: string } = $props();
</script>

<div class="workspace">
  <svelte:boundary>
    <RecordingHistory {project} />{#snippet pending()}<p>
        Loading recordings…
      </p>{/snippet}
  </svelte:boundary>
  {#if id}<RecordingTranscript {project} {id} />{:else}<section>
      <h1>Project workspace</h1>
      <p>
        Select a recorded conversation, or start recording with the macOS
        controls.
      </p>
    </section>{/if}
  <svelte:boundary>
    <FileLibrary {project} />{#snippet pending()}<p>Loading files…</p>{/snippet}
  </svelte:boundary>
</div>

<style>
  .workspace {
    display: flex;
    height: 100%;
    min-height: 0;
  }
  section {
    flex: 1;
    padding: 1rem;
    min-width: 0;
  }
  h1 {
    font-size: 1.5rem;
  }
  @media (max-width: 850px) {
    .workspace {
      flex-wrap: wrap;
    }
  }
</style>
