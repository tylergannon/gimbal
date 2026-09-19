<script lang="ts">
  import { Skeleton } from "#lib/components/ui/skeleton/index.js";
</script>

<section class="runs-loading" aria-labelledby="runs-loading-title" aria-busy="true">
  <header>
    <h1 id="runs-loading-title">Runs</h1>
    <p role="status" aria-live="polite">Loading runs…</p>
  </header>

  <div class="tools" aria-hidden="true">
    <Skeleton class="loading-filters" />
    <Skeleton class="loading-search" />
  </div>

  <div class="cards" aria-hidden="true">
    {#each [0, 1] as card}
      <div class="run-card" data-loading-card={card}>
        <div class="card-head">
          <div class="identity">
            <Skeleton class="loading-name" />
            <Skeleton class="loading-run-id" />
          </div>
          <Skeleton class="loading-status" />
        </div>
        <div class="latest">
          <Skeleton class="loading-activity" />
          <Skeleton class="loading-summary" />
        </div>
        <Skeleton class="loading-instruction" />
        <div class="card-foot">
          <Skeleton class="loading-stats" />
          <Skeleton class="loading-open" />
        </div>
      </div>
    {/each}
  </div>
</section>

<style>
  .runs-loading {
    box-sizing: border-box;
    width: min(1120px, 100%);
    padding: 36px 0 24px;
    color: var(--foreground);
  }

  header {
    margin-bottom: 28px;
  }

  h1,
  p {
    margin: 0;
  }

  h1 {
    font-size: 24px;
    font-weight: 600;
    line-height: 32px;
    letter-spacing: -0.025em;
  }

  p {
    color: var(--status-muted);
  }

  .tools,
  .card-head,
  .card-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .tools {
    margin-bottom: 12px;
  }

  .runs-loading :global(.loading-filters) {
    width: 490px;
    max-width: 58%;
    height: 36px;
  }

  .runs-loading :global(.loading-search) {
    width: 300px;
    height: 36px;
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 14px;
  }

  .run-card {
    display: flex;
    min-width: 0;
    flex-direction: column;
    gap: 16px;
    padding: 16px;
    background: var(--card);
    border: 1px solid var(--map-line);
    border-radius: var(--radius);
    box-shadow: 0 1px 2px rgb(0 0 0 / 6%);
  }

  .identity,
  .latest {
    display: flex;
    min-width: 0;
    flex-direction: column;
  }

  .identity {
    width: 52%;
    gap: 6px;
  }

  .latest {
    gap: 8px;
  }

  .runs-loading :global(.loading-name) {
    width: 70%;
    height: 20px;
  }

  .runs-loading :global(.loading-run-id) {
    width: 100%;
    height: 14px;
  }

  .runs-loading :global(.loading-status) {
    width: 82px;
    height: 24px;
    border-radius: 999px;
  }

  .runs-loading :global(.loading-activity) {
    width: 100%;
    height: 16px;
  }

  .runs-loading :global(.loading-summary) {
    width: 78%;
    height: 18px;
  }

  .runs-loading :global(.loading-instruction) {
    width: 100%;
    height: 65px;
  }

  .runs-loading :global(.loading-stats) {
    width: 64%;
    height: 34px;
  }

  .runs-loading :global(.loading-open) {
    width: 70px;
    height: 16px;
  }

  @media (max-width: 900px) {
    .runs-loading {
      padding: 24px 16px;
    }

    .tools {
      align-items: stretch;
      flex-direction: column;
    }

    .runs-loading :global(.loading-filters),
    .runs-loading :global(.loading-search) {
      width: 100%;
      max-width: none;
    }
  }

  @media (max-width: 760px) {
    .cards {
      grid-template-columns: 1fr;
    }

    .run-card:nth-child(2) {
      display: none;
    }
  }
</style>
