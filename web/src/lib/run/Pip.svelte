<script module lang="ts">
  export type PipState = "queued" | "running" | "needs-you" | "ended" | "failed" | "cancelled";
</script>

<script lang="ts">
  let { state }: { state: PipState } = $props();
</script>

<span class="pip" data-state={state} aria-hidden="true"></span>

<style>
  .pip {
    position: relative;
    display: inline-block;
    width: 8px;
    height: 8px;
    box-sizing: border-box;
    flex-shrink: 0;
    border-radius: 9999px;
  }

  .pip[data-state="ended"] {
    background: var(--foreground);
  }

  .pip[data-state="running"] {
    border: 2px solid var(--status-running);
    border-top-color: transparent;
    animation: spin 1s linear infinite;
  }

  .pip[data-state="failed"] {
    background: var(--destructive);
  }

  .pip[data-state="failed"]::before,
  .pip[data-state="failed"]::after {
    position: absolute;
    top: 3px;
    right: 2px;
    left: 2px;
    height: 1.5px;
    background: var(--background);
    content: "";
    transform: rotate(45deg);
  }

  .pip[data-state="failed"]::after {
    transform: rotate(-45deg);
  }

  .pip[data-state="needs-you"] {
    width: 9px; height: 9px;
    background: var(--background);
    box-shadow: inset 0 0 0 2px var(--status-needs-you);
  }

  .pip[data-state="needs-you"]::after {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 3px;
    height: 3px;
    border-radius: 9999px;
    background: var(--status-needs-you);
    content: "";
  }

  .pip[data-state="queued"] {
    border: 1.5px dashed var(--status-muted);
  }

  .pip[data-state="cancelled"] { border: 1px solid var(--foreground); background: linear-gradient(90deg, var(--foreground) 50%, transparent 50%); }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
