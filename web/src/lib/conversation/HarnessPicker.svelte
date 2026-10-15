<script lang="ts">
  import CheckIcon from '@lucide/svelte/icons/check';
  import ChevronDownIcon from '@lucide/svelte/icons/chevron-down';
  import * as Popover from '#lib/components/ui/popover';
  import type { Setup } from '#lib/skgo/conversation/types';
  import { harnessMark, harnessName, modelName } from './names';

  type Harness = Setup['harnesses'][number];
  type Choice = { provider: string; model: string; effort?: string };
  let { harnesses, choice, onpick, states = [], machine = 'This Mac' }: {
    harnesses: Harness[]; choice: Choice; onpick: (choice: Choice) => void;
    states?: Setup['harness_states']; machine?: string;
  } = $props();
  const unavailable = (id: string) => states.find(item => item.id === id && (item.state === 'missing' || item.state === 'sign-in-required'));
  const reason = (id: string) => unavailable(id)?.state === 'missing' ? `Not installed on ${machine}` : `Sign-in required on ${machine}`;
  let open = $state(false);
  const shown = $derived(harnesses.find(item => item.id === choice.provider) ?? harnesses[0]);
  const models = $derived(shown.models.includes(choice.model) || shown.id !== choice.provider ? shown.models : [choice.model, ...shown.models]);
  const efforts = $derived(shown.efforts?.find(item => item.model === choice.model)?.levels ?? []);
</script>

<Popover.Root bind:open>
  <Popover.Trigger class="chip" aria-label="Harness and model">
    <span class="mark" aria-hidden="true">{harnessMark(choice.provider)}</span>
    <span class="chip-label">{harnessName(choice.provider)} · {modelName(choice.model)}{choice.effort ? ` · ${choice.effort}` : ''}</span>
    <ChevronDownIcon size={14} aria-hidden="true" />
  </Popover.Trigger>
  <Popover.Content align="start" collisionPadding={8} class="w-[min(560px,calc(100vw-32px))] gap-0 p-0">
    <div class="columns">
      <div class="column" role="group" aria-label="Harness">
        <h3>Harness</h3>
        {#each harnesses as harness (harness.id)}
          <button type="button" class={["option", { selected: harness.id === choice.provider }]}
            disabled={!!unavailable(harness.id)} title={unavailable(harness.id) ? reason(harness.id) : undefined}
            onclick={() => { if (harness.id !== choice.provider) onpick({ provider: harness.id, model: harness.default, effort: '' }); }}>
            <span class="mark" aria-hidden="true">{harnessMark(harness.id)}</span>
            <span class="label">{harnessName(harness.id)}{#if unavailable(harness.id)}<small>{reason(harness.id)}</small>{/if}</span>
            {#if harness.id === choice.provider}<CheckIcon size={14} aria-hidden="true" />{/if}
          </button>
        {/each}
      </div>
      <div class="column models" role="group" aria-label={`${harnessName(shown.id)} models`}>
        <h3>{harnessName(shown.id)} models</h3>
        {#each models as model (model)}
          <button type="button" class={["option", { selected: model === choice.model }]} disabled={!!unavailable(shown.id)}
            onclick={() => { onpick({ provider: shown.id, model, effort: '' }); open = false; }}>
            <span class="label">{modelName(model)}<code>{model}</code></span>
            {#if model === choice.model}<CheckIcon size={14} aria-hidden="true" />{/if}
          </button>
        {/each}
      </div>
    </div>
    <footer><label>Effort <select aria-label="Effort" value={choice.effort || ''} onchange={(event) => onpick({ ...choice, effort: event.currentTarget.value })} disabled={!efforts.length}><option value="">Default</option>{#each efforts as level (level)}<option value={level}>{level.charAt(0).toUpperCase() + level.slice(1)}</option>{/each}</select></label><p>{efforts.length ? 'Applies to every turn in this session.' : 'Effort follows this model.'}</p></footer>
  </Popover.Content>
</Popover.Root>

<style>
  .columns { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr); }
  .column { display: grid; align-content: start; gap: 2px; padding: 6px; }
  .models { border-left: 1px solid var(--border); }
  h3 { margin: 4px 6px 6px; color: var(--muted-foreground); font-size: 12px; font-weight: 400; }
  .option { display: flex; align-items: center; gap: 10px; width: 100%; min-height: 36px; padding: 4px 8px; border: 0; border-radius: 6px; color: inherit; background: none; font: inherit; font-size: 13px; text-align: left; cursor: pointer; }
  .option:disabled { opacity: .5; cursor: default; }
  small { font-size: 11px; color: var(--muted-foreground); }
  footer { border-top: 1px solid var(--border); padding: 10px 14px; font-size: 12px; }
  footer label { display: flex; align-items: center; justify-content: space-between; }
  footer select { border: 0; background: var(--muted); color: inherit; border-radius: 5px; padding: 4px 8px; font: inherit; }
  footer p { margin: 6px 0 0; color: var(--muted-foreground); font-size: 11px; }
  .option:hover, .option.selected { background: var(--muted); }
  .label { display: grid; flex: 1; min-width: 0; }
  code { overflow: hidden; color: var(--muted-foreground); font-family: var(--font-mono); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
  .mark { display: grid; flex: none; place-items: center; width: 18px; height: 18px; border-radius: 4px; color: var(--primary-foreground); background: var(--primary); font-size: 10px; font-weight: 600; }
</style>
