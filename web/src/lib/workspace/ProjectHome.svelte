<script lang="ts">
  import MessageSquare from '@lucide/svelte/icons/message-square';
  import { conversationSetup } from '../../routes/conversation.remote';
  import type { Conversation } from '#lib/skgo/conversation/types';
  import type { Header } from '#lib/skgo/transcript/types';
  import type { InteractiveRow } from '#lib/observation';
  import type { RunCardItem } from '#lib/run/RunsList.svelte';
  import * as Card from '#lib/components/ui/card';
  import * as Table from '#lib/components/ui/table';
  import { projectActivity, statusLabel } from './projectHome';
  import WorkflowLauncher from './WorkflowLauncher.svelte';
  import Pip from '#lib/run/Pip.svelte';
  let { project, name, path, sessions, recordings, runs, error, onretry, active = true, onopen, onopenguide }: {
    project:string; name:string; path:string; sessions:Conversation[]; recordings:Header[];
    runs?:{items:RunCardItem[];attention:InteractiveRow[];now:number}; error?:unknown; onretry?:()=>void; active?:boolean;
    onopen:(path:string)=>void; onopenguide:(name:string)=>void;
  } = $props();
  const setup = $derived(conversationSetup({project,environment:'local'}));
  const entries = $derived(projectActivity(project,sessions,runs?.items ?? [],runs?.attention ?? [],recordings));
  const running = $derived(entries.filter(item => item.status === 'running' || item.status === 'queued'));
  const waiting = $derived(entries.filter(item => item.status === 'needs-you'));
  const failed = $derived(entries.filter(item => item.status === 'failed' && new Date(item.updated).toDateString() === new Date(runs?.now || Date.now()).toDateString()));
  let filter = $state('All');
  const recent = $derived(entries.filter(item => filter === 'All' || item.kind === filter));
  const when = (ms:number) => new Date(ms).toDateString() === new Date().toDateString() ? new Date(ms).toLocaleTimeString(undefined,{hour:'2-digit',minute:'2-digit'}) : new Date(ms).toLocaleDateString(undefined,{month:'short',day:'numeric'});
</script>
<section class="home" aria-label="Project home">
  <header class="page-head"><h1>{name}</h1><span class="facts">{path}{setup.current?.branch ? ` · ${setup.current.branch}` : ''}{setup.current?.machine ? ` · ${setup.current.machine}` : ''}</span><div class="spacer"></div><WorkflowLauncher align="end" {project} {active} {onopen} {onopenguide} /><a class="new-session" href={`/projects/${project}/conversations/new`}><MessageSquare size={14} />New session</a></header>
  {#if error}<p class="quiet" role="alert">Could not read run status. <button onclick={onretry}>Retry</button></p>{/if}
  <div class="now3">
    {#each [{title:'Running',state:'running' as const,items:running,description:`${running.length} things live now`},{title:'Needs you',state:'needs-you' as const,items:waiting,description:`${waiting.length} ${waiting.length === 1 ? 'item needs' : 'items need'} your attention`},{title:'Failed today',state:'failed' as const,items:failed,description:`${failed.filter(item => item.kind === 'Runs').length} runs, ${failed.filter(item => item.kind === 'Sessions').length} sessions`}] as group (group.title)}
      <Card.Root size="sm" class="status-card"><Card.Header><Card.Title class="card-title"><Pip state={group.state} />{group.title}</Card.Title><Card.Description>{runs ? group.description : 'Loading status…'}</Card.Description></Card.Header><Card.Content class="status-content">
        {#each group.items.slice(0,4) as item (item.path)}<div class="now-item"><a href={item.path}><Pip state={item.status} /><b>{item.title}</b></a>{#if item.question}<p class="question">“{item.question}”</p><a class="answer" href={item.path}>Answer</a>{:else if item.detail}<p class="quiet">{item.detail}</p>{/if}</div>{:else}{#if runs}<p class="quiet">{group.state === 'running' ? 'Nothing running right now.' : group.state === 'needs-you' ? 'Nothing waiting on you.' : 'No failures today.'}</p>{/if}{/each}
        {#if group.items.length > 4}<a class="quiet" href={`/projects/${project}/runs`}>See all {group.items.length} items</a>{/if}
      </Card.Content></Card.Root>
    {/each}
  </div>
  <Card.Root class="recent-card"><Card.Header><Card.Title>Pick up where you left off</Card.Title><Card.Description>Everything in this project, most recent first.</Card.Description><Card.Action><div class="filters" role="group" aria-label="Recent activity filter">{#each ['All','Sessions','Runs','Recordings'] as value (value)}<button aria-pressed={filter === value} onclick={() => filter = value}>{value}</button>{/each}</div></Card.Action></Card.Header><Card.Content>
    <Table.Root><Table.Header><Table.Row><Table.Head>What</Table.Head><Table.Head>Status</Table.Head><Table.Head>Bound to</Table.Head><Table.Head class="when">When</Table.Head></Table.Row></Table.Header><Table.Body>{#each recent as item (item.path)}<Table.Row><Table.Cell class="what"><a href={item.path}><b>{item.title}</b><span>{item.kind === 'Sessions' ? 'session' : item.kind === 'Runs' ? 'run' : 'recording'}</span></a></Table.Cell><Table.Cell><span class="record-status"><Pip state={item.status} />{item.kind === 'Recordings' && item.status === 'ended' ? 'Stopped' : statusLabel(item.status)}</span></Table.Cell><Table.Cell class="binding">{item.bound || '—'}</Table.Cell><Table.Cell class="when">{when(item.updated)}</Table.Cell></Table.Row>{:else}<Table.Row><Table.Cell colspan={4} class="empty">{filter === 'All' ? 'No activity yet. Start a session or run a workflow.' : `No ${filter.toLowerCase()} yet.`}</Table.Cell></Table.Row>{/each}</Table.Body></Table.Root>
  </Card.Content></Card.Root>
</section>
<style>
  .home { height:100%; overflow:auto; padding:24px 32px; box-sizing:border-box; color:var(--foreground); font:14px/20px var(--font-sans); } .page-head { display:flex; align-items:center; flex-wrap:wrap; gap:12px; margin-bottom:16px; } h1 { margin:0; font:600 20px/26px var(--font-sans); } .facts { font:12px/18px var(--font-mono); color:var(--muted-foreground); overflow-wrap:anywhere; } .spacer { flex:1; } .new-session,.answer { display:inline-flex; align-items:center; justify-content:center; gap:7px; padding:7px 11px; border:1px solid var(--foreground); border-radius:6px; background:var(--foreground); color:var(--background); text-decoration:none; font-size:13px; }
  .now3 { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:16px; margin-bottom:16px; } :global(.status-card) { min-height:210px; box-shadow:none; border:1px solid var(--border); } :global(.card-title) { display:flex; align-items:center; gap:6px; font-weight:500; font-size:13px; } :global(.status-content) { display:flex; flex-direction:column; gap:12px; } .now-item { min-width:0; } .now-item>a:first-child { display:flex; align-items:center; gap:8px; color:var(--foreground); text-decoration:none; font-size:14px; } .now-item b { font-weight:600; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; } .now-item p { margin:6px 0 0 20px; font-size:13px; line-height:19px; overflow-wrap:anywhere; display:-webkit-box; -webkit-line-clamp:2; line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; } .quiet { margin:0; color:var(--muted-foreground); font-size:13px; } .answer { margin:10px 0 0 20px; padding:5px 9px; } .question { color:var(--muted-foreground); }
  :global(.recent-card) { box-shadow:none; border:1px solid var(--border); } .filters { display:flex; padding:3px; border-radius:8px; background:var(--muted); } .filters button { padding:4px 10px; color:var(--muted-foreground); background:transparent; border:0; border-radius:5px; font:13px/18px var(--font-sans); cursor:pointer; } .filters button[aria-pressed=true] { color:var(--foreground); background:var(--background); box-shadow:0 1px 3px #0001; } :global(.what) { width:36%; white-space:normal; } :global(.what a) { display:flex; flex-direction:column; color:var(--foreground); text-decoration:none; } :global(.what a span) { color:var(--muted-foreground); font-size:13px; } :global(.binding) { font-size:12px; color:var(--muted-foreground); max-width:300px; white-space:normal; overflow-wrap:anywhere; } :global(.when) { text-align:right; color:var(--muted-foreground); font:12px var(--font-mono); } .record-status { display:inline-flex; align-items:center; gap:6px; font-size:13px; white-space:nowrap; } :global(.empty) { padding:24px; color:var(--muted-foreground); text-align:center; }
  @media(max-width:950px) { .home { padding:20px; } .now3 { grid-template-columns:1fr; } :global(.status-card) { min-height:0; } :global(.recent-card [data-slot=card-header]) { display:flex; flex-wrap:wrap; } }
</style>
