<script lang="ts">
  import { tick, untrack } from 'svelte';
  import Play from '@lucide/svelte/icons/play';
  import MapIcon from '@lucide/svelte/icons/map';
  import { harnessName } from '#lib/conversation/names';
  import { environmentSnapshot } from '#lib/environments.remote';
  import type { RunSnapshot, RunObservation } from '#lib/observation';
  import type { Graph } from '#lib/workflow/types';
  import { buildMapLayout } from './layout';
  import { selectedInstancesFor } from './selection';
  import Pip from './Pip.svelte';
  import RunTurn from './RunTurn.svelte';
  import LoopComposer from './LoopComposer.svelte';
  import RunQuestions from './RunQuestions.svelte';
  import SteerBox from './detail/SteerBox.svelte';
  import Payload from './detail/Payload.svelte';
  import type { MapSelection } from './types';
  let { project, snapshot, observation, revision, graph, onmap }: {
    project: string; snapshot: RunSnapshot; observation: RunObservation; revision: number; graph?: Graph;
    onmap: (selection?: MapSelection) => void;
  } = $props();
  let picked = $state('');
  const live = $derived(snapshot.run.status === 'running');
  const sessions = $derived(Object.values(snapshot.sessions).sort((a,b) => a.created - b.created));
  const waiting = $derived(Object.values(snapshot.interviews).filter(row => row.status === 'pending' && live));
  const activeTurn = $derived(Object.values(snapshot.turns).filter(row => !row.ended).sort((a,b) => b.started-a.started)[0]);
  const chosen = $derived(sessions.find(row => row.id === picked) ?? sessions.find(row => row.id === waiting[0]?.session || row.name === waiting[0]?.session) ?? sessions.find(row => row.id === activeTurn?.session) ?? sessions.at(-1));
  const turns = $derived(Object.values(snapshot.turns).filter(row => row.session === chosen?.id).sort((a,b) => a.started - b.started));
  const running = $derived(turns.find(row => !row.ended));
  const question = $derived(waiting.find(row => row.session === chosen?.id || row.session === chosen?.name) ?? (waiting.length === 1 && !waiting[0].session ? waiting[0] : undefined));
  const layout = $derived(graph ? buildMapLayout(graph, snapshot, { selectedInstances: selectedInstancesFor(running?.scope || turns.at(-1)?.scope || chosen?.scope || '') }) : undefined);
  const plannerScopes = $derived(layout?.sheets.filter(sheet => sheet.kind === 'loop' && sheet.scope.status === 'running' && layout?.loops.some(item => item.loop.name === sheet.scope.name && item.loop.planner === chosen?.name)) ?? []);
  const plannerScope = $derived(live && !running && plannerScopes.length === 1 ? plannerScopes[0].scope : undefined);
  const inventory = $derived(environmentSnapshot(project));
  const target = $derived(inventory.current?.environments.find(item => item.environment.id === (snapshot.run.task_environment || 'local')));
  const canSteer = $derived(target?.capabilities.steer === true);
  const steerReason = $derived(!live ? 'This run has ended.' : !running ? 'Waiting for the next turn. Steering opens when this session starts working.' : inventory.error ? 'Could not check steering availability.' : !inventory.current ? 'Checking steering availability…' : !canSteer ? 'This environment cannot be steered.' : 'Read at the next model call in the running turn. Dropped if the turn ends first.');
  const invocation = $derived(snapshot.scopes['']);
  const taskText = (value: unknown): string => typeof value === 'string' ? value : value && typeof value === 'object' && 'description' in value ? String(value.description) : value === undefined ? '' : JSON.stringify(value);
  const time = (value: number) => new Date(value).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  const stateFor = (id: string) => waiting.some(row => row.session === id || row.session === snapshot.sessions[id]?.name) ? 'waiting' as const : Object.values(snapshot.turns).some(row => row.session === id && !row.ended && live) ? 'running' as const : Object.values(snapshot.turns).some(row => row.session === id && row.error && !row.interrupted) ? 'failed' as const : 'ended' as const;
  const timeline = $derived([
    ...turns.map(turn => ({ kind: 'turn' as const, at: turn.started, id: turn.id, turn })),
    ...(chosen?.steers ?? []).map((steer, index) => ({ kind: 'steer' as const, at: steer.created, id: `steer-${index}`, steer })),
    ...Object.values(snapshot.commands).map(command => ({ kind: 'command' as const, at: command.started, id: `command-${command.id}`, command })),
    ...Object.values(snapshot.scopes).filter(scope => scope.key).map(scope => ({ kind: 'scope' as const, at: scope.began, id: `scope-${scope.key}`, scope })),
    ...Object.values(snapshot.interviews).filter(row => row.session === chosen?.id || row.session === chosen?.name || (!row.session && waiting.length === 1)).map(row => ({ kind: 'interview' as const, at: row.asked, id: row.question_id, question: row })),
  ].sort((a,b) => a.at-b.at));
  let scroller = $state<HTMLDivElement>();
  let follow = $state(false);
  // Keep a reader's position; only follow incoming content while already at the tail.
  $effect.pre(() => { revision; const target = scroller; if (target && untrack(() => follow)) void tick().then(() => { target.scrollTop = target.scrollHeight; }); });
</script>
<div class="run-transcript">
  <div class="measure">
    {#if layout}
      <div class="orient" aria-label="Where this session sits in the workflow">
        {#each layout.sheets.filter(sheet => sheet.scope.key && (running?.scope || turns.at(-1)?.scope || '').startsWith(sheet.scope.key)) as sheet (sheet.selectionKey)}<span class="group-label">{sheet.kind} · {sheet.scope.key}</span>{/each}
        {#each layout.nodes as node (node.selectionKey)}
          <button class={['step', { now: node.state === 'running', wait: node.state === 'waiting' }]} onclick={() => onmap({ kind: 'node', scope: snapshot.scopes[node.scopeKey], operation: node.operation, runtime: node.runtime })}>
            <span class="glyph">{node.operation.kind === 'command' ? '›_' : node.operation.kind === 'interview' ? '◇' : '▱'}</span>{node.operation.kind === 'agent_call' ? node.operation.session : node.operation.name}<Pip state={node.state} />
          </button><span class="arrow">›</span>
        {/each}
        <button class="map-button" onclick={() => onmap()}><MapIcon size={13} />Map</button>
      </div>
    {/if}
    <div class="switcher" aria-label="Sessions in this run">
      {#each sessions as session (session.id)}<button aria-pressed={chosen?.id === session.id} class={{ selected: chosen?.id === session.id }} onclick={() => { picked = session.id; }}><Pip state={stateFor(session.id)} />{session.name}<span class="quiet">· {Object.values(snapshot.turns).filter(row => row.session === session.id).length} turns</span></button>{/each}
    </div>
    {#if chosen}<div class="identities"><span>{harnessName(chosen.adapter.replace(/^\*/, '').replace(/\.adapter$/i, ''))}</span><span>{chosen.model}</span>{#if chosen.effort}<span>{chosen.effort}</span>{/if}{#if chosen.workdir}<span class="workdir" title={chosen.workdir}>{chosen.workdir}</span>{/if}</div>{/if}
  </div>
  <div class="scroll" bind:this={scroller} onscroll={() => { if (scroller) follow = scroller.scrollHeight-scroller.scrollTop-scroller.clientHeight < 48; }}>
    <div class="transcript measure">
      <section class="invoke" aria-label="Invocation">
        <div class="ih"><Play size={14} />Invocation <span class="quiet">· {time(snapshot.run.started)}</span></div>
        {#if invocation?.task || invocation?.values.goal?.value}<p class="goal">{taskText(invocation.task ?? invocation?.values.goal?.value)}</p>{/if}
        <dl><dt>workflow</dt><dd>{snapshot.run.name}</dd>{#if snapshot.run.task_environment || chosen?.workdir}<dt>where</dt><dd>{snapshot.run.task_environment || 'local'}{chosen?.workdir ? ` · ${chosen.workdir}` : ''}</dd>{/if}<dt>models</dt><dd>{sessions.map(session => `${session.name} ${session.model}${session.effort ? ` ${session.effort}` : ''}`).join(' · ') || 'No sessions recorded yet'}</dd></dl>
        {#if invocation && Object.keys(invocation.values).some(name => name !== 'goal' && name !== 'result')}<details><summary>Recorded root context</summary>{#each Object.entries(invocation.values).filter(([name]) => name !== 'goal' && name !== 'result') as [name, entry] (name)}<Payload label={name} text={entry.artifact?.preview || taskText(entry.value)} />{/each}</details>{/if}
      </section>
      {#each timeline as item (item.id)}
        {#if item.kind === 'turn'}<RunTurn turn={item.turn} ordinal={turns.findIndex(row => row.id === item.turn.id)+1} {snapshot} {observation} {revision} />
        {:else if item.kind === 'steer'}<article class="steer"><div class="turn-head">Steer{item.steer.source ? ` · ${item.steer.source}` : ''} · {time(item.at)} · {item.steer.landed ? 'Landed' : 'Dropped'}</div><p class="bubble">{item.steer.message}</p></article>
        {:else if item.kind === 'command'}<details class="run-event"><summary><Pip state={!item.command.ended ? 'running' : item.command.exit_code && !item.command.interrupted ? 'failed' : 'ended'} /><b>{item.command.name}</b><span>{item.command.ended ? `exit ${item.command.exit_code}` : 'running'}</span><span class="line"></span><span>{time(item.at)}</span></summary><Payload label="command" text={`${item.command.command} ${item.command.args.join(' ')}\n${item.command.stderr || item.command.stdout}`} /></details>
        {:else if item.kind === 'scope'}<div class="run-event"><Pip state={item.scope.error ? 'failed' : item.scope.status === 'running' ? 'running' : 'ended'} /><b>{item.scope.name || item.scope.key}</b><span>{item.scope.status}</span><span class="line"></span><span>{time(item.at)}</span></div>
        {:else if item.kind === 'interview'}<section class={['interview', { answered: item.question.status === 'answered' }]}><div class="ih"><Pip state={item.question.status === 'pending' ? 'waiting' : 'ended'} />{item.question.status === 'pending' ? 'Needs you' : 'Answered'}<span class="quiet">· {item.question.name} · asked {time(item.at)}</span></div><p>{item.question.question}</p>{#if item.question.status === 'answered'}<p class="answer">{item.question.answer || 'Interview ended.'}</p>{/if}</section>{/if}
      {/each}
      {#if !chosen}<p class="quiet">Waiting for the workflow to start a session…</p>{/if}
      {#if snapshot.run.error}<p class="error">{snapshot.run.error}</p>{/if}
    </div>
  </div>
  {#if !follow}<button class="jump" onclick={() => { if (scroller) scroller.scrollTop = scroller.scrollHeight; follow = true; }}>Jump to latest ↓</button>{/if}
  <div class="composer measure">
    {#if question}<RunQuestions run={snapshot.run.id} {question} composer target={chosen?.name} />
    {:else if chosen && plannerScope}<LoopComposer run={snapshot.run.id} scope={plannerScope.key} name={chosen.name} />
    {:else if chosen}<SteerBox run={snapshot.run.id} session={chosen.id} target={!live ? 'This run has ended' : !running ? `${chosen.name} · waiting for the next turn` : chosen.name} turn={running?.id} ordinal={running ? turns.indexOf(running)+1 : undefined} ended={!live || !running} disabled={!canSteer} reason={steerReason} />{/if}
  </div>
</div>
<style>
  .run-transcript { position:relative; display:flex; flex:1; min-height:0; min-width:0; flex-direction:column; } .measure { width:100%; max-width:860px; box-sizing:border-box; margin:0 auto; } .orient { display:flex; align-items:center; flex-wrap:wrap; gap:6px; padding:8px 10px; border:1px solid var(--border); border-radius:6px; background:color-mix(in oklch,var(--muted) 18%,var(--background)); font:12px/16px var(--font-sans); } .step { display:inline-flex; align-items:center; gap:5px; height:24px; padding:0 8px; border:1px solid var(--border); border-radius:3px; background:var(--background); color:var(--foreground); font:inherit; white-space:nowrap; cursor:pointer; } .step.now { outline:2px solid var(--status-running); outline-offset:1px; } .step.wait { outline:2px solid var(--status-needs-you); outline-offset:1px; } .glyph { color:var(--status-running); } .arrow { color:var(--muted-foreground); } .group-label { font-weight:500; color:var(--muted-foreground); } .map-button { margin-left:auto; display:flex; align-items:center; gap:4px; border:0; background:transparent; font:inherit; cursor:pointer; color:var(--foreground); }
  .switcher { display:flex; align-items:center; margin-top:12px; overflow-x:auto; border-bottom:1px solid var(--border); } .switcher button { display:flex; align-items:center; gap:6px; flex:none; padding:10px 12px; color:var(--muted-foreground); background:transparent; border:0; border-bottom:2px solid transparent; font:13px/18px var(--font-sans); cursor:pointer; } .switcher button.selected { color:var(--foreground); border-bottom-color:var(--foreground); } .quiet { color:var(--muted-foreground); font-weight:400; font-size:12px; } .identities { display:flex; gap:6px; padding:10px 0 0; flex-wrap:wrap; } .identities span { padding:2px 7px; font-size:11px; border:1px solid var(--border); border-radius:4px; color:var(--muted-foreground); } .workdir { overflow:hidden; text-overflow:ellipsis; max-width:320px; white-space:nowrap; }
  .scroll { flex:1; min-height:0; overflow:auto; } .transcript { display:flex; flex-direction:column; gap:24px; padding-top:16px; padding-bottom:24px; } .invoke { display:flex; flex-direction:column; gap:8px; max-width:760px; padding:12px 14px; border:1px solid var(--border); border-radius:8px; background:color-mix(in oklch,var(--muted) 18%,var(--background)); } .ih { display:flex; align-items:center; flex-wrap:wrap; gap:8px; font:500 13px/18px var(--font-sans); } .goal { margin:0; font:15px/22px var(--font-sans); } dl { display:grid; grid-template-columns:max-content 1fr; gap:2px 14px; margin:0; font:13px/20px var(--font-sans); } dt { color:var(--muted-foreground); } dd { margin:0; overflow-wrap:anywhere; } details { font-size:12px; color:var(--muted-foreground); } summary { cursor:pointer; }
  .steer { display:flex; flex-direction:column; align-items:flex-end; gap:8px; } .turn-head { color:var(--muted-foreground); font:12px/16px var(--font-sans); } .bubble { margin:0; padding:12px 16px; max-width:80%; border:1px solid var(--border); border-radius:12px; font:15px/22px var(--font-sans); white-space:pre-wrap; } .run-event, .run-event summary { display:flex; align-items:center; gap:8px; color:var(--muted-foreground); font:13px/18px var(--font-sans); } details.run-event { display:block; } b { font-weight:500; color:var(--foreground); } .line { flex:1; height:1px; background:var(--border); }
  .interview { display:flex; flex-direction:column; gap:8px; max-width:640px; padding:12px 14px; border:1.5px solid var(--status-needs-you); border-radius:6px; background:color-mix(in oklch,var(--status-needs-you) 8%,var(--background)); } .interview.answered { border-color:var(--border); background:var(--background); } .interview p { margin:0; font:15px/22px var(--font-sans); } .answer { color:var(--muted-foreground); }
  .composer { flex:none; padding-bottom:10px; border-top:1px solid var(--border); } .composer :global(.steer-box) { padding:12px 0 0; } .composer :global(form) { max-width:none; } .composer :global(.question) { border:0; padding:12px 0; } .composer :global(.label) { color:var(--foreground); font-size:13px; font-weight:500; } .jump { position:absolute; bottom:150px; left:50%; transform:translateX(-50%); padding:6px 12px; background:var(--background); color:var(--foreground); border:1px solid var(--border); border-radius:6px; font-size:12px; cursor:pointer; } .error { color:var(--destructive); font-size:13px; }
  @media(max-width:1000px) { .measure { padding-left:24px; padding-right:24px; } .transcript { padding-top:16px; } .composer :global(.steer-box) { padding-left:0; padding-right:0; } }
</style>
