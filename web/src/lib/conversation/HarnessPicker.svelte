<script lang="ts">
  import CheckIcon from '@lucide/svelte/icons/check';
  import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
  import * as Popover from '#lib/components/ui/popover';
  import type { Setup } from '#lib/skgo/conversation/types';
  import { harnessMark, harnessName, modelName } from './names';

  type Harness = Setup['harnesses'][number];
  type Choice = { provider: string; model: string };
  let { harnesses, choice, onpick }: { harnesses: Harness[]; choice: Choice; onpick: (choice: Choice) => void } = $props();
  let open = $state(false);
  const shown = $derived(harnesses.find(item => item.id === choice.provider) ?? harnesses[0]);
  const models = $derived(shown.models.includes(choice.model) || shown.id !== choice.provider ? shown.models : [choice.model, ...shown.models]);
</script>

<Popover.Root bind:open>
  <Popover.Trigger class="chip" aria-label="Harness and model">
    <span class="mark">{harnessMark(choice.provider)}</span>
    <span class="chip-label">{harnessName(choice.provider)} · {modelName(choice.model)}</span>
    <ChevronDownIcon size={14} aria-hidden="true" />
  </Popover.Trigger>
  <Popover.Content align="start" collisionPadding={8} class="w-[min(560px,calc(100vw-32px))] gap-0 p-0">
    <div class="columns">
      <div class="column" role="group" aria-label="Harness">
        <h3>Harness</h3>
        {#each harnesses as harness (harness.id)}
          <button type="button" class={["option", { selected: harness.id === choice.provider }]}
            onclick={() => { if (harness.id !== choice.provider) onpick({ provider: harness.id, model: harness.default }); }}>
            <span class="mark">{harnessMark(harness.id)}</span>
            <span class="label">{harnessName(harness.id)}</span>
            {#if harness.id === choice.provider}<CheckIcon size={14} aria-hidden="true" />{/if}
          </button>
        {/each}
      </div>
      <div class="column models" role="group" aria-label={`${harnessName(shown.id)} models`}>
        <h3>{harnessName(shown.id)} models</h3>
        {#each models as model (model)}
          <button type="button" class={["option", { selected: model === choice.model }]}
            onclick={() => { onpick({ provider: shown.id, model }); open = false; }}>
            <span class="label">{modelName(model)}<code>{model}</code></span>
            {#if model === choice.model}<CheckIcon size={14} aria-hidden="true" />{/if}
          </button>
        {/each}
      </div>
    </div>
  </Popover.Content>
</Popover.Root>

<style>
  .columns { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr); }
  .column { display: grid; align-content: start; gap: 2px; padding: 6px; }
  .models { border-left: 1px solid var(--border); }
  h3 { margin: 4px 6px 6px; color: var(--muted-foreground); font-size: 12px; font-weight: 400; }
  .option { display: flex; align-items: center; gap: 10px; width: 100%; min-height: 36px; padding: 4px 8px; border: 0; border-radius: 6px; color: inherit; background: none; font: inherit; font-size: 13px; text-align: left; cursor: pointer; }
  .option:hover, .option.selected { background: var(--muted); }
  .label { display: grid; flex: 1; min-width: 0; }
  code { overflow: hidden; color: var(--muted-foreground); font-family: var(--font-mono); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
  .mark { display: grid; flex: none; place-items: center; width: 18px; height: 18px; border-radius: 4px; color: var(--primary-foreground); background: var(--primary); font-size: 10px; font-weight: 600; }
</style>
